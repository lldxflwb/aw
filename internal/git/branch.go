package git

// BranchDelete deletes a branch. If force is true, uses -D instead of -d.
func BranchDelete(repoDir, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := GitRun(repoDir, "branch", flag, branch)
	return err
}
