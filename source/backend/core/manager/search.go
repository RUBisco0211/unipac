package manager

// MaxSearchResults is how many packages one manager may contribute to a
// search. A keyword like "a" matches thousands of them in Homebrew, and the
// table shows a few dozen at a time: past this, the list costs more than it
// tells, and the backend says it was cut rather than cutting it in silence.
const MaxSearchResults = 200

// SearchResult is what a search returned, and whether a manager had more than
// MaxSearchResults to give.
type SearchResult struct {
	Packages  []Package
	Truncated bool
}
