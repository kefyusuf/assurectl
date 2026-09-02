package domain

type Subject struct {
	RepositoryURI string `json:"repository_uri"`
	BaseRevision  string `json:"base_revision"`
	HeadRevision  string `json:"head_revision"`
	ChangeSetAlgo string `json:"change_set_algorithm"`
	ChangeSetHash string `json:"change_set_digest"`
}
