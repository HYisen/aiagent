package main

import (
	"aiagent/clients/chat"
	"aiagent/clients/openai"
	"aiagent/clients/session"
	"aiagent/service"
	"bufio"
	"context"
	_ "embed"
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
	"strconv"
	"strings"
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
	// following code lacks index creation, but shall work in a lower level.
	// db.AutoMigrate(&model.Chat{}, &model.Session{}, &model.Result{})

	if err := db.Exec(ddlSQL).Error; err != nil {
		log.Fatal(err)
	}
}

func server() {
	client := openai.New("https://api.deepseek.com", *DeepSeekAPIKey)
	db, err := gorm.Open(sqlite.Open(sqliteDatabaseFilename))
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
	s := service.New(client, sr, cr)
	local, err := url.Parse(fmt.Sprintf("http://localhost:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	wf.SetTimeout(30 * time.Second) // LLM is relative slow.
	err = http.ListenAndServe(local.Host, s)
	if err != nil {
		log.Fatal(err)
	}
}

func repl() {
	client := openai.New("https://api.deepseek.com", *DeepSeekAPIKey)
	var history []openai.Message

	scanner := bufio.NewScanner(os.Stdin)
	const escapeLine = "exit()"
	fmt.Printf("hint: input %s to escape\n", escapeLine)
	for scanner.Scan() {
		line := scanner.Text()
		if line == escapeLine {
			break
		}
		log.Printf("read line : %s\n", line)
		history = append(history, openai.NewUserMessage(line))
		fmt.Printf("sending with history size %d\n", len(history))
		cc, err := client.OneShotStream(context.Background(), openai.Request{
			Messages: history,
			Model:    openai.ChatModelDeepSeekR1,
		}, NewPrintWordChannel())
		if err != nil {
			log.Fatal(err)
		}
		history = append(history, cc.Choices[0].Message.HistoryRecord())
	}
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
	aggregated, err := client.OneShotStream(context.Background(), req, NewPrintItemChannel())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\naggregated:\n%+v\n", aggregated)
}

func NewPrintItemChannel() chan<- openai.ChatCompletionChunk {
	ret := make(chan openai.ChatCompletionChunk)
	go func(ch <-chan openai.ChatCompletionChunk) {
		for chunk := range ch {
			fmt.Printf("%+v\n", chunk)
		}
		log.Println("end of print channel")
	}(ret)
	return ret
}

func NewPrintWordChannel() chan<- openai.ChatCompletionChunk {
	ret := make(chan openai.ChatCompletionChunk)
	go func(ch <-chan openai.ChatCompletionChunk) {
		var count int
		var cotFinished bool
		for chunk := range ch {
			count++
			if count == 1 {
				deltaT := time.Now().Unix() - chunk.Created
				msg := "created at T0 on server, now on client it's T"
				if deltaT > 0 {
					msg += "+"
				} else {
					msg += "-"
				}
				msg += strconv.FormatInt(deltaT, 10)
				msg += "s"
				log.Println(msg)

				fmt.Printf("%+v\n", chunk.ChatCompletionBase)
				continue
			}

			delta := chunk.Choices[0].Delta
			if delta.Role != "" {
				fmt.Printf("role: %s\n", delta.Role)
			}
			if delta.ReasoningContent != "" {
				fmt.Print(delta.ReasoningContent)
			}
			if !cotFinished && delta.ReasoningContent == "" && delta.Content != "" {
				cotFinished = true
				separation := strings.Repeat("-", 8)
				fmt.Print("\n\n" + separation + " CoT END " + separation + "\n\n")
			}
			if delta.Content != "" {
				fmt.Print(delta.Content)
			}

			if chunk.Usage != nil {
				fmt.Println()
				fmt.Printf("FinishReason: %s\n", *chunk.Choices[0].FinishReason)
				fmt.Printf("Usage: %+v\n", *chunk.Usage)
				continue
			}
		}
		log.Printf("end of PrintWordChannel, total useful trunk %d\n", count)
	}(ret)
	return ret
}
