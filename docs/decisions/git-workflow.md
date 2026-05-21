# Git Workflow

## atomic commit

Everything as code, all related comes together in one commit,
so they can be referred or reverted as one later.

You are encouraged to squash continuous commits that have same motivation.
(e.g. To `fix` a bug, there should be `test` to reproduce, `refactoring` for scaffolding,
`feature` as API updates and `docs` to introduce the change.
One can and even shall commit those changes separately,
just if they are continuous commits, squash them to one commit.)

Consequently, [conventional-commits](https://www.conventionalcommits.org)
**is not** applied in this project. Those tag info can be normally digested from commit message,
and you are allowed to add them explicitly into commit header/body/footer as long as finding them useful.

If you think the refactoring commit alone is acceptable even if the feature/bugfix is denied in the end,
leave it in separated commits.

Don't fix multiple bugs or deliver multiple features in one commit, unless they are tied.
(i.e., an **and** word in commit message is a typical signal that you have messed up.)

## main release

The main/master branch is preserved for release.

Make your best to keep every new state on that branch operational.
(e.g., compilable, documented, test all passed, linter-warning-free)

Such restriction has reduced effect on other branches and no effect on
volatile(i.e., not planned to be included in PR) commit.

Thus, use Pull-Request, don't fast-forward merge into main.

## dev on named branch

Use its task-name as branch name, making it easier to smell a context switch.
`New Branch`, `Merge`, `Cherry-Pick` whenever terms and conditions applied.

Despite I want every branch bound to an issue. I don't want to record them.
So branch is allowed as related features with bug-fixes that would be shipped together as
a git tag controlled version release. I shall control branch growth on size and time.

At present, we don't do squash merge on PR.(have one exception below)
Because I decide to store separated commits data in `git`,
not Pull-Request or Code Review features hosted by platforms such likes GitHub.

We value the eventual outcome. If it's a trivial bug-fix, you could have a dedicated branch, do conventional-commits
and squash merge on PR. That is similar to have one polished commit as PR and end with a non-squashed merge.
If you don't think one squashed commit satisfied atomcity, then it is not trivial.

Try not solve conflict on merge, git pull and rebase your branch onto remote changed main right before PR instead.
Use `git merge --no-ff your-branch-name`, introduce your Change List in the merge commit message.
For human, you can do this on web GUI and as an opportunity to Code Review.

## respect history

Unless reviewed and approved, don't `push -f` on release branch.

You shall hesitate to alter history that has been public (i.e., pushed).

It's okay to leave a fix commit several commits after its feature implementation.
One (trying) fix may not actually fix. You can leave those evidences here.

## monorepo?

In commit message, scope `client: ` is used to indicates this Change List happens **only** on and for `tools/client`.

If a commit changed both client and server or elsewhere, don't mention the dedicated scope as it's mixed.

Maybe some day I would decide to pull that binary artifact related code out form this git repository.

Other scope keyword are not used yet. Update this document if you plan to add one.