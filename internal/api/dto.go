// Package api is the Go mirror of the /api/v1 contract: the DTO shapes
// defined in src/lib/api-v1/dto.ts, the error envelope in
// src/lib/api-auth/errors.ts, and an HTTP client skeleton that talks to
// them. cli/testdata/api/*.json are validated against the TS Zod schemas by
// src/lib/api-v1/contract.test.ts, so the field names and nullability here
// must stay byte-for-byte in sync with dto.ts.
package api

// MatchSource mirrors the MATCH_SOURCES tuple in src/lib/api-v1/dto.ts.
type MatchSource string

const (
	MatchSourceTag      MatchSource = "tag"
	MatchSourceNote     MatchSource = "note"
	MatchSourceKeyword  MatchSource = "keyword"
	MatchSourceSemantic MatchSource = "semantic"
)

// ContentKind mirrors CONTENT_KINDS in src/lib/bookmarks/types.ts.
type ContentKind string

const (
	ContentKindArticle ContentKind = "article"
	ContentKindDocs    ContentKind = "docs"
	ContentKindRepo    ContentKind = "repo"
	ContentKindProfile ContentKind = "profile"
	ContentKindFeed    ContentKind = "feed"
	ContentKindVideo   ContentKind = "video"
	ContentKindOther   ContentKind = "other"
)

// BookmarkDto mirrors BookmarkDtoSchema (src/lib/api-v1/dto.ts). It never
// carries content_text, embeddings or note bodies.
type BookmarkDto struct {
	ID              string        `json:"id"`
	URL             string        `json:"url"`
	Title           *string       `json:"title"`
	Summary         *string       `json:"summary"`
	Tags            []string      `json:"tags"`
	ReadTimeMinutes *int          `json:"readTimeMinutes"`
	ReadLater       bool          `json:"readLater"`
	ContentKind     *ContentKind  `json:"contentKind"`
	MatchedVia      []MatchSource `json:"matchedVia"`
	CreatedAt       string        `json:"createdAt"`
}

// BookmarkListDto mirrors BookmarkListDtoSchema: items ranked and sliced to
// `limit`, `total` counted before slicing, `truncated` when total > limit.
type BookmarkListDto struct {
	Items     []BookmarkDto `json:"items"`
	Total     int           `json:"total"`
	Truncated bool          `json:"truncated"`
}

// SharedItemDto mirrors SharedItemDtoSchema. Unlike BookmarkDto, notes are
// included here -- they were explicitly shared.
type SharedItemDto struct {
	ShareID    string   `json:"shareId"`
	URL        string   `json:"url"`
	Title      *string  `json:"title"`
	Summary    *string  `json:"summary"`
	Tags       []string `json:"tags"`
	SharedBy   *string  `json:"sharedBy"`
	SharedAt   string   `json:"sharedAt"`
	Notes      []string `json:"notes"`
	MarkistURL string   `json:"markistUrl"`
}

// SharedListDto mirrors SharedListDtoSchema.
type SharedListDto struct {
	Items []SharedItemDto `json:"items"`
}

// MeDto mirrors MeDtoSchema.
type MeDto struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}
