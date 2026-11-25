package main

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func posting(date, desc, account string, amt float64) Txn {
	return Txn{Date: day(date), Desc: desc, To: account, Cur: amt}
}

func charge(date, desc, account string, amt float64) Txn {
	return Txn{Date: day(date), Desc: desc, From: account, Cur: amt}
}

func TestRemoveDuplicatesRepeatPurchaseOnAdjacentDay(t *testing.T) {
	const card = "Liabilities:Credit:Fidelity"
	p := &parser{txns: []Txn{
		posting("2026-09-17", "JUST SALAD", "Expenses:Food", 16.32),
		posting("2026-09-17", "JUST SALAD", card, -16.32),
	}}
	got := p.removeDuplicates([]Txn{
		charge("2026-09-17", "JUST SALAD", card, -16.32),
		charge("2026-09-18", "JUST SALAD", card, -16.32),
	})
	if len(got) != 1 || !got[0].Date.Equal(day("2026-09-18")) {
		t.Fatalf("want only the 09-18 charge kept, got %+v", got)
	}
}

func TestRemoveDuplicatesExactDatePairedFirst(t *testing.T) {
	const card = "Liabilities:Credit:Fidelity"
	p := &parser{txns: []Txn{
		posting("2026-09-18", "JUST SALAD", card, -16.32),
	}}
	got := p.removeDuplicates([]Txn{
		charge("2026-09-17", "JUST SALAD", card, -16.32),
		charge("2026-09-18", "JUST SALAD", card, -16.32),
	})
	if len(got) != 1 || !got[0].Date.Equal(day("2026-09-17")) {
		t.Fatalf("want only the 09-17 charge kept, got %+v", got)
	}
}

func TestRemoveDuplicatesIgnoresOtherAccounts(t *testing.T) {
	p := &parser{txns: []Txn{
		posting("2026-09-17", "JUST SALAD", "Liabilities:Credit:AMEXPlatinum", -16.32),
	}}
	got := p.removeDuplicates([]Txn{
		charge("2026-09-17", "JUST SALAD", "Liabilities:Credit:Fidelity", -16.32),
	})
	if len(got) != 1 {
		t.Fatalf("charge on another card must not count as a duplicate, got %+v", got)
	}
}

func TestRemoveDuplicatesWithinWindow(t *testing.T) {
	const card = "Liabilities:Credit:Fidelity"
	p := &parser{txns: []Txn{
		posting("2026-09-17", "JUST SALAD", card, -16.32),
	}}
	got := p.removeDuplicates([]Txn{
		charge("2026-09-18", "JUST SALAD", card, -16.32),
		charge("2026-09-20", "CHIPOTLE", card, -12.52),
	})
	if len(got) != 1 || got[0].Desc != "CHIPOTLE" {
		t.Fatalf("want posting-date drift absorbed and new txn kept, got %+v", got)
	}
}
