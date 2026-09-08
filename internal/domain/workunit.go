package domain

type WorkUnitType string

const (
	WorkUnitPullRequest         WorkUnitType = "pull_request"
	WorkUnitCommit              WorkUnitType = "commit"
	WorkUnitReleaseCandidate    WorkUnitType = "release_candidate"
	WorkUnitAPIMigration        WorkUnitType = "api_migration"
	WorkUnitSDKMigration        WorkUnitType = "sdk_migration"
	WorkUnitFrameworkUpgrade    WorkUnitType = "framework_upgrade"
	WorkUnitDependencyUpgrade   WorkUnitType = "dependency_upgrade"
	WorkUnitSecurityRemediation WorkUnitType = "security_remediation"
)

func (v WorkUnitType) Valid() bool {
	switch v {
	case WorkUnitPullRequest,
		WorkUnitCommit,
		WorkUnitReleaseCandidate,
		WorkUnitAPIMigration,
		WorkUnitSDKMigration,
		WorkUnitFrameworkUpgrade,
		WorkUnitDependencyUpgrade,
		WorkUnitSecurityRemediation:
		return true
	default:
		return false
	}
}

type WorkUnit struct {
	ID        string       `json:"id"`
	Type      WorkUnitType `json:"type"`
	Objective string       `json:"objective"`
}
