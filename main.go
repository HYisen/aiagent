package main

import (
	"aiagent/clients/chat"
	"aiagent/clients/openai"
	"aiagent/clients/session"
	"aiagent/console"
	"aiagent/service"
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"github.com/hyisen/wf"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"time"
)

var DeepSeekAPIKey = flag.String("DeepSeekAPIKey", "this_is_a_secret", "API Key from platform.deepseek.com/api_keys")

var mode = flag.String("mode", "server", "app mode from SmokeTest|REPL|server|migrate")

var port = flag.Int("port", 8640, "where server mode serve on localhost")

func main() {
	flag.Parse()
	switch *mode {
	case "REPL":
		repl()
	case "SmokeTest":
		basic()
	case "server":
		server()
	case "migrate":
		migrate()
	default:
		log.Fatalf("unsupported mode %s", *mode)
	}
}

const sqliteDatabaseFilename = "db"

//go:embed docs/ddl.sql
var ddlSQL string

func migrate() {
	_, err := os.Stat(sqliteDatabaseFilename)
	if err == nil {
		log.Fatalf("SQLite file [%s] already exists, backup and remove it first.", sqliteDatabaseFilename)
	}

	slog.Info("create sqlite database", "path", sqliteDatabaseFilename)
	db, err := gorm.Open(sqlite.Open(sqliteDatabaseFilename))
	if err != nil {
		log.Fatal(err)
	}

	// migrate from go source code works, but I choose to leave it in SQL, making it more friendly to developers.
	// the following code lacks index creation but shall work in a lower level.
	// db.AutoMigrate(&model.Chat{}, &model.Session{}, &model.Result{})

	if err := db.Exec(ddlSQL).Error; err != nil {
		log.Fatal(err)
	}
}

func server() {
	client := openai.New("https://api.deepseek.com", *DeepSeekAPIKey)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("%s?_foreign_keys=on", sqliteDatabaseFilename)))
	if err != nil {
		log.Fatal(err)
	}
	sr, err := session.NewRepository(db)
	if err != nil {
		log.Fatal(err)
	}
	cr, err := chat.NewRepository(db)
	if err != nil {
		log.Fatal(err)
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		log.Fatal("no build info")
	}
	s := service.New(client, sr, cr, bi)
	local, err := url.Parse(fmt.Sprintf("http://localhost:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	// Notably, chat APIs timeout is controlled separately else where.
	// As I sampled, it's normally about 900 ms,
	// so we extend the default timeout to 2000 ms to cover most cases.
	wf.SetTimeout(2000 * time.Millisecond)
	err = http.ListenAndServe(local.Host, s)
	if err != nil {
		log.Fatal(err)
	}
}

type REPLLineHandler struct {
	history []openai.Message
	client  *openai.Client
}

func NewREPLLineHandler(client *openai.Client) *REPLLineHandler {
	return &REPLLineHandler{client: client}
}

func (h *REPLLineHandler) HandleLine(line string) {
	h.history = append(h.history, openai.NewUserMessage(line))
	fmt.Printf("sending with history size %d\n", len(h.history))
	cc, err := h.client.OneShotStream(context.Background(), openai.Request{
		Messages: h.history,
		Model:    openai.ChatModelDeepSeekR1,
	}, console.NewPrintWordChannel())
	if err != nil {
		if errors.Is(err, openai.ErrUpstream) {
			log.Printf("Poped question history because of upstream error: %v", err)
			h.history = h.history[:len(h.history)-1]
			return
		}
		log.Fatal(err)
	}
	h.history = append(h.history, cc.Choices[0].Message.HistoryRecord())
}

func repl() {
	client := openai.New("https://api.deepseek.com", *DeepSeekAPIKey)
	handler := NewREPLLineHandler(client)
	controller := console.NewController(handler, console.NewDefaultOptions())
	controller.Run()
}

func basic() {
	client := openai.New("https://api.deepseek.com", *DeepSeekAPIKey)
	req := openai.Request{
		Messages: []openai.Message{{
			Role:    "user",
			Content: "say this is a test",
		}},
		Model: openai.ChatModelDeepSeekR1,
	}

	rsp, err := client.OneShot(context.Background(), req)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", rsp)

	fmt.Println("\nnext stream")
	aggregated, err := client.OneShotStream(context.Background(), req, console.NewPrintItemChannel())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\naggregated:\n%+v\n", aggregated)
}
