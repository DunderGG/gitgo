package app

// RepoInfo holds high-level information about the currently opened repository.
type RepoInfo struct {
	Path        string `json:"path"`
	Branch      string `json:"branch"`
	HasRemote   bool   `json:"hasRemote"`
	HasUpstream bool   `json:"hasUpstream"`
}

// CommitSummary is a lightweight representation of a commit for the commit list.
// IsUnpushed indicates whether the commit is safe to edit.
type CommitSummary struct {
	Hash       string `json:"hash"`
	ShortHash  string `json:"shortHash"`
	Message    string `json:"message"`
	Author     string `json:"author"`
	Date       string `json:"date"`
	IsUnpushed bool   `json:"isUnpushed"`
}

// CommitDetail carries the full metadata needed to populate the edit panel.
type CommitDetail struct {
	Hash        string `json:"hash"`
	Message     string `json:"message"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
	Date        string `json:"date"`
	IsUnpushed  bool   `json:"isUnpushed"`
}

// EditRequest describes the desired change to a commit's metadata.
type EditRequest struct {
	Hash        string `json:"hash"`
	Message     string `json:"message"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
	Date        string `json:"date"`
}

// OperationResult is returned by all mutating bound methods to convey
// success or failure to the frontend.
type OperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
