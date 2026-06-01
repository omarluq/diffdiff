package git

import (
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

// shortHashLen is the number of leading hex characters shown for a commit hash.
const shortHashLen = 7

// Details summarizes the repository's current HEAD for display in the UI.
type Details struct {
	// Branch is the current branch name, or "" when HEAD is detached or unborn.
	Branch string
	// Head is the short HEAD commit hash, or "" when there are no commits.
	Head string
	// Subject is the first line of the HEAD commit message.
	Subject string
}

// Details returns the current HEAD branch, short hash, and commit subject. An
// unborn HEAD (a repository with no commits) yields the zero Details.
func (r *Repository) Details() Details {
	ref, err := r.repo.Head()
	if err != nil {
		return Details{Branch: "", Head: "", Subject: ""}
	}

	details := Details{Branch: "", Head: shortHash(ref.Hash()), Subject: ""}
	if ref.Name().IsBranch() {
		details.Branch = ref.Name().Short()
	}

	if commit, err := r.repo.CommitObject(ref.Hash()); err == nil {
		details.Subject = firstLine(commit.Message)
	}

	return details
}

// shortHash abbreviates a commit hash to its leading characters.
func shortHash(hash plumbing.Hash) string {
	full := hash.String()
	if len(full) > shortHashLen {
		return full[:shortHashLen]
	}

	return full
}

// firstLine returns the first line of a commit message, trimmed.
func firstLine(message string) string {
	if before, _, ok := strings.Cut(message, "\n"); ok {
		return strings.TrimSpace(before)
	}

	return strings.TrimSpace(message)
}
