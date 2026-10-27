package main

import (
	"io"
	"strings"
	"testing"
)

func TestStripIncompatibleSettings(t *testing.T) {
	// Data lines after the preamble must survive verbatim, even one that looks
	// exactly like the stripped setting.
	in := `--
-- PostgreSQL database dump
--

\restrict abc123

SET statement_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SELECT pg_catalog.set_config('search_path', '', false);

CREATE TABLE public.t (v text);
COPY public.t (v) FROM stdin;
SET transaction_timeout = 0;
\.
`
	want := strings.Replace(in, "SET transaction_timeout = 0;\n", "", 1)

	r, err := stripIncompatibleSettings(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestStripIncompatibleSettingsPreambleOnly(t *testing.T) {
	// A dump that ends inside the preamble (no trailing newline) is passed through.
	in := "SET lock_timeout = 0;\nSET transaction_timeout = 0;"
	r, err := stripIncompatibleSettings(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(r)
	if string(got) != "SET lock_timeout = 0;\n" {
		t.Errorf("got %q", got)
	}
}
