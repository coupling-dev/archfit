package requestsend

// Repository identifies a github.com repository from its Git remote URL.
func Repository(remote string) string {
	return githubRepository(remote)
}
