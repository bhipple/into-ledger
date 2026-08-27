package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// existingAccounts holds the accounts already present in the journal before
// this run, used to populate the fuzzy account selector.
var existingAccounts = newStringSet()

// stringSet is a minimal set of strings. Only the operations the fuzzy
// selectors need are implemented.
type stringSet map[string]struct{}

func newStringSet(items ...string) stringSet {
	s := make(stringSet, len(items))
	for _, i := range items {
		s.Add(i)
	}
	return s
}

// Add inserts an item. Empty strings are ignored, so that the trailing blank
// line of a command's output does not become a blank entry in the fuzzy menu.
func (s stringSet) Add(i string) {
	if len(i) == 0 {
		return
	}
	s[i] = struct{}{}
}

func (s stringSet) Contains(i string) bool {
	_, has := s[i]
	return has
}

// ToSlice returns the items in sorted order, so the fuzzy menu is stable
// between runs.
func (s stringSet) ToSlice() []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func runCommandRaw(name string, arg ...string) string {
	cmd := exec.Command(name, arg...)
	out, err := cmd.Output()
	checkf(err, "Error running `%v`: `%v`", name, arg)
	return string(out)
}

// runCommand executes the given cmd and returns the list of lines printed on
// stdout.
func runCommand(name string, arg ...string) []string {
	out := runCommandRaw(name, arg...)
	return strings.Split(out, "\n")
}

// Fzf holds parameters for github.com/junegunn/fzf
type Fzf struct {
	// Items to be selected by user
	Items []string
	// If prompt is set "", a default prompt is used.
	Prompt string
	//  Used to fill the search field
	Query string
	// Returns what the user searched for
	ReturnQuery bool
	// More arguments to the fzf command
	MoreArgs []string
}

// fuzzySelect prompts the user to select one or more items in a fuzzy menu.
func fuzzySelect(f Fzf) (selected []string) {
	args := []string{}
	if f.Prompt != "" {
		args = append(args, "--prompt", f.Prompt+" >")
	}
	if f.ReturnQuery {
		// Print what the user entered first-line
		args = append(args, "--print-query")
	}
	args = append(args, "--query", f.Query)
	// Default options, TODO Make this configurable
	args = append(args, "--height", "80%", "--layout=reverse")
	// Other options
	args = append(args, f.MoreArgs...)
	// Inspired from https://stackoverflow.com/a/23167416/ by mraron (Apache
	// 2.0 licence)
	subProcess := exec.Command("fzf", args...)
	stdin, err := subProcess.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdout, err := subProcess.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	subProcess.Stderr = os.Stderr
	err = subProcess.Start()
	checkf(err, "Error running fzf. Is https://github.com/junegunn/fzf installed?")
	io.WriteString(stdin, strings.Join(f.Items, "\n"))
	stdin.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(stdout)
	s := buf.String()
	subProcess.Wait()
	for i, s := range strings.Split(s, "\n") {
		switch {
		// Always keep user query even if it is empty
		case f.ReturnQuery && i == 0:
			selected = append(selected, s)
		case s != "":
			selected = append(selected, s)
		}
	}
	return
}

// fuzzySelectAccount selects an account with an external fuzzy finder, showing
// the account's balance and register in the preview pane. Returns an empty
// string if there is no match.
func fuzzySelectAccount(accounts *stringSet) string {
	selected := fuzzySelect(Fzf{
		Items: accounts.ToSlice(),
		MoreArgs: []string{
			"--preview", "ledger b --force-color {}; ledger r --force-color {}",
			"--preview-window", "right:60%",
		},
	})
	if len(selected) == 0 {
		return ""
	}
	// TODO Support multiple accounts?
	return selected[0]
}

// assignAccount files the account on the correct side of the transaction,
// according to the sign of the amount.
func assignAccount(t *Txn, accountName string) {
	if t.Cur > 0 {
		t.From = accountName
	} else {
		t.To = accountName
	}
}

// fuzzyAndGetResult lets the user pick any existing account via fzf. It is the
// fallback when the keyboard shortcuts don't cover the account you want.
func (p *parser) fuzzyAndGetResult(accounts *stringSet, t *Txn) float64 {
	assignAccount(t, fuzzySelectAccount(accounts))
	if len(t.To) > 0 && len(t.From) > 0 {
		p.writeToDB(*t)
		t.Done = true
		return 1.0
	}
	return 0
}
