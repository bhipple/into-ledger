package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	yaml "gopkg.in/yaml.v2"
)

var payeeSubstitutionsFile = flag.String("payeesubst", "payee_subst.yaml",
	"Name of the payee substitutions file, read from the config directory.")

// PayeeSubstitutions maps a payee as it appears in the CSV to the name to use
// in the journal. It is maintained by hand; into-ledger only reads it.
type PayeeSubstitutions map[string]string

// performPayeeSubstitution renames payees that have an entry in the
// substitutions file. Payees without one are left alone.
func performPayeeSubstitution(txns []Txn) {
	fpath := path.Join(*configDir, *payeeSubstitutionsFile)
	data, err := os.ReadFile(fpath)
	if err != nil {
		if *debug {
			fmt.Printf("No payee substitution file found at %v\n", fpath)
		}
		return
	}

	subst := make(PayeeSubstitutions)
	checkf(yaml.Unmarshal(data, &subst), "Unable to unmarshal yaml config at %v", fpath)

	var count int
	for i := range txns {
		if replacement, has := subst[txns[i].Desc]; has {
			txns[i].Desc = replacement
			count++
		}
	}
	if count > 0 {
		fmt.Printf("\t%d payees substituted.\n\n", count)
	}
}
