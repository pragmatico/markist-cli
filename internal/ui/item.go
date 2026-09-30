package ui

import (
	"fmt"
	"strings"
)

// Item is a single row the picker and plain renderer show (plan Task 13): a
// search/list result or a shared item, flattened into the fields both
// renderers need. The field is ItemTitle rather than Title because Title()
// is also the method DefaultItem requires bubbles/list to render a row.
type Item struct {
	ItemTitle    string
	URL          string
	SecondaryURL string // shared's markistUrl; empty for search/list rows
	Hostname     string
	Tags         []string
	ReadMinutes  *int
	MatchedLabel string   // "Matched: tag" etc; empty when no query is active
	Meta         string   // shared rows: "@sharer · 3d ago"
	Notes        []string // shared rows with --notes; nil otherwise
}

// Title satisfies bubbles/list's DefaultItem interface.
func (i Item) Title() string { return i.ItemTitle }

// FilterValue satisfies bubbles/list's Item interface: what "/" fuzzy-filters
// against.
func (i Item) FilterValue() string {
	parts := append([]string{i.ItemTitle, i.Hostname}, i.Tags...)
	return strings.Join(parts, " ")
}

// Description satisfies bubbles/list's DefaultItem interface: the row's
// second line -- hostname, tags, read time, the "Matched: …" label, and (for
// shared rows) the sharer/timestamp meta and notes, whichever are set.
func (i Item) Description() string {
	return strings.Join(i.metaParts(), " · ")
}

func (i Item) metaParts() []string {
	var parts []string
	if i.Hostname != "" {
		parts = append(parts, i.Hostname)
	}
	if len(i.Tags) > 0 {
		tags := make([]string, len(i.Tags))
		for j, t := range i.Tags {
			tags[j] = "#" + t
		}
		parts = append(parts, strings.Join(tags, " "))
	}
	if i.ReadMinutes != nil {
		parts = append(parts, fmt.Sprintf("~%d min", *i.ReadMinutes))
	}
	if i.Meta != "" {
		parts = append(parts, i.Meta)
	}
	if i.MatchedLabel != "" {
		parts = append(parts, i.MatchedLabel)
	}
	if len(i.Notes) > 0 {
		parts = append(parts, "notes: "+strings.Join(i.Notes, " | "))
	}
	return parts
}
