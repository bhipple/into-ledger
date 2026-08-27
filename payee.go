package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

var payeeSubstitutionsFile = flag.String("payeesubst", "payee_subst.yaml",
	"Name of payee substitutions file.")

var (
	// existingPayees holds the payees already present in the journal before
	// this run, used to populate the fuzzy payee selector.
	existingPayees = newStringSet()

	payeeSubsts = make(PayeeSubstitutions)
)

// listPayee asks ledger for the payees already used in the journal.
// TODO Make this asynchronous
func listPayee() stringSet {
	ledgerPayeeCmd := []string{"ledger", "-f", *journal, "payees"}
	fmt.Printf("Getting payee list with: `%v`\n", ledgerPayeeCmd)
	return newStringSet(runCommand(ledgerPayeeCmd[0], ledgerPayeeCmd[1:]...)...)
}

// PayeeSubstitutions holds all the replacements from source payee (the key in
// the map) to the desired payee name (the value in the map)
type PayeeSubstitutions map[string]string

// Persist saves the payee substitutions to a yaml file
func (ps *PayeeSubstitutions) Persist(path string) {
	data, err := yaml.Marshal(*ps)
	if err != nil {
		log.Fatalf("marshal payee substitutions: %v", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatalf("While writing payee substitutions to file '%v': %v", path, err)
	}
}

// performPayeeSubstitution renames payees that are keys of subst. For payees
// that are neither already in the journal nor already substituted, it displays
// a fuzzy selection menu to pick the canonical name.
func performPayeeSubstitution(txns []Txn, subst PayeeSubstitutions, payees *stringSet) {
	// Continuously select with fuzzy menu, without asking
	fuzzyContinuous := false
TxnLoop:
	for i := range txns {
		txn := &txns[i]
		payee := txn.Desc
		if payees.Contains(payee) {
			continue
		}

		if replacement, has := subst[payee]; has {
			txn.Desc = replacement
			continue
		}

		answer := "f"
		if !fuzzyContinuous {
			answer = askPayeeQuestion(fmt.Sprintf(
				"Unknown payee: '%v' ([F]uzzy select/Fuzzy select [a]ll/[i]gnore/ig[n]ore all): ",
				payee), "fain", "f")
		}
		switch answer {
		case "n":
			break TxnLoop

		case "i":
			continue TxnLoop

		case "a":
			fuzzyContinuous = true
			fallthrough
		case "f":
			fuzzySelectUpdateTxn(txn, subst, payee, payees)
		}
	}
}

// askPayeeQuestion prompts user with question and loops while it doesn’t
// get an answer in choices. Empty string or line-return goes to defaultChoice
func askPayeeQuestion(question string, choices string, defaultChoice string) string {
	var answer string
	for {
		fmt.Print(question)
		b := make([]byte, 1)
		n, err := os.Stdin.Read(b)
		fmt.Println()
		if err != nil || n == 0 {
			// stdin is closed, so no answer will ever arrive. Take the
			// default rather than spinning forever.
			return defaultChoice
		}
		answer = strings.ToLower(string(b))
		if answer == "" || answer == "\n" {
			return defaultChoice
		}
		for _, c := range choices {
			if string(c) == answer {
				return answer
			}
		}
	}
}

// fuzzySelectUpdateTxn asks the user for the canonical payee name, records the
// substitution so it is applied automatically next time, and updates the txn.
func fuzzySelectUpdateTxn(txn *Txn, subst PayeeSubstitutions, payee string, payees *stringSet) {
	// selected is like []string{"user entered query", "result 1", "result 2", …}
	selected := fuzzySelect(Fzf{
		Items:       payees.ToSlice(),
		Prompt:      payee,
		Query:       strings.ToLower(payee),
		ReturnQuery: true,
	})

	var replacement string
	// If there were one or more results, we want to keep the first one, since
	// we can’t have multiple payees for a transaction.
	if len(selected) > 1 {
		replacement = selected[1]
	} else if len(selected) == 1 {
		// Since fuzzySelect inserts the query as first element, the slice
		// normally has at least one element.
		replacement = selected[0]
		fmt.Println("Nothing selected, replacement:", replacement)
	}

	// We keep the existing payee if there is nothing better to replace it with.
	if replacement != "" {
		fmt.Println("Replacing with:", replacement)
		subst[payee] = replacement
		txn.Desc = replacement
	} else {
		fmt.Println("Not replaced")
	}
}
