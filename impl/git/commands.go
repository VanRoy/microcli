package git

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thoas/go-funk"
	"github.com/urfave/cli/v3"
	"github.com/vanroy/microcli/impl/cmd"
	"github.com/vanroy/microcli/impl/config"
	"github.com/vanroy/microcli/impl/file"
	"github.com/vanroy/microcli/impl/glob"
	"github.com/vanroy/microcli/impl/initialzr"
	"github.com/vanroy/microcli/impl/prompt"
)

const (
	MAX_DEPTH = 2
)

type GitCommands struct {
	config     config.Config
	impl       gitRemote
	initializr *initialzr.Initializr
}

type GitImplement struct {
	Id   string
	Name string
	Impl func(config config.Config) gitRemote
}

var gitImplements = []GitImplement{
	{Id: "github", Name: "GitHub", Impl: newGitHub},
	{Id: "gitlab", Name: "GitLab", Impl: newGitLab},
	{Id: "azure", Name: "Azure DevOps", Impl: newAzure},
}

func NewCommands(config config.Config) *GitCommands {

	return &GitCommands{
		config:     config,
		impl:       getImpl(config),
		initializr: initialzr.NewInitializr(config),
	}
}

// Init config
func Init(_ context.Context, c *cli.Command) error {

	typeOptions := funk.Map(gitImplements, func(impl GitImplement) prompt.Option {
		return prompt.Option{Id: impl.Id, Name: impl.Name}
	}).([]prompt.Option)

	gitType := prompt.Choice("Select your server type :", typeOptions)
	prompt.PrintNewLine()

	gitTypeName := getImplCfg(gitType).Name
	baseUrl := prompt.Input(fmt.Sprintf("Enter your %s base url :", gitTypeName))

	var authMode = "pat"
	var token string
	if gitType == "azure" {
		azAuthMode := []prompt.Option{
			{Id: "az-cli", Name: "Azure CLI"},
			{Id: "pat", Name: "Personal access token"},
		}
		authMode = prompt.Choice("Select your authentication mode :", azAuthMode)
		if authMode == "pat" {
			token = prompt.Password(fmt.Sprintf("Enter your %s token :", gitTypeName))
		}
	} else {
		token = prompt.Password(fmt.Sprintf("Enter your %s token :", gitTypeName))
	}

	prompt.PrintNewLine()

	tmpConfig := config.Config{
		Git: config.GitConfig{
			Type:         gitType,
			BaseUrl:      baseUrl,
			PrivateToken: token,
			AuthMode:     authMode,
		},
	}

	impl := getImpl(tmpConfig)
	groups, err := impl.getGroups()
	if err != nil {
		prompt.PrintErrorf("Cannot retrieve %s (%s)", impl.getLabels().GroupsLabel, err.Error())
		os.Exit(1)
	}
	if len(groups) == 0 {
		prompt.PrintErrorf("Cannot retrieve %s", impl.getLabels().GroupsLabel)
		os.Exit(1)
	}

	groupOptions := funk.Map(groups, func(group gitGroup) prompt.Option {
		return prompt.Option{Id: group.Id, Name: group.Name}
	}).([]prompt.Option)

	prompt.PrintNewLine()
	groupId := prompt.Choice("Select group to add on workspace :", groupOptions)
	prompt.PrintNewLine()

	tmpConfig.Git.GroupIds = []string{groupId}

	protocolOptions := []prompt.Option{
		{Id: "ssh", Name: "SSH"},
		{Id: "https", Name: "HTTPS"},
	}

	protocol := prompt.Choice("Select the protocol used for cloning repositories :", protocolOptions)
	prompt.PrintNewLine()

	tmpConfig.Git.CloneProtocol = protocol

	if protocol == "https" {
		useTokenOpen := []prompt.Option{
			{Id: "token", Name: "Use token for GIT operation"},
			{Id: "cm", Name: "Use GIT integrated authentication ( git-credentials-manager)"},
		}
		useToken := prompt.Choice("Select the protocol used for cloning repositories :", useTokenOpen)
		prompt.PrintNewLine()
		tmpConfig.Git.UseTokenForOperation = useToken == "token"
	}

	config.Save(tmpConfig)

	return nil
}

// Refresh authentication config
func Auth(_ context.Context, c *cli.Command) error {

	existingConfig, _ := config.Load()

	token := prompt.Password(fmt.Sprintf("Enter the new %s token :", getImplCfg(existingConfig.Git.Type).Name))
	prompt.PrintNewLine()

	existingConfig.Git.PrivateToken = token

	config.SavePassword(existingConfig.Git)

	return nil
}

// Return impl labels
func (g *GitCommands) GetLabels() Labels {
	return g.impl.getLabels()
}

// Return list of GIT repository present in local folder
func (g *GitCommands) ListLocal(_ context.Context, c *cli.Command) error {

	folders, err := getGitFolders("", c.StringArgs("exclude"))
	if err != nil {
		return err
	}

	if len(folders) == 0 {
		fmt.Printf("No ")
	}

	funk.ForEach(folders, func(folder string) {
		fmt.Printf("* %s\n", folder)
	})

	return nil
}

// Return list of GIT repository present on remote server
func (g *GitCommands) ListRemote(_ context.Context, c *cli.Command) error {

	repos, err := g.impl.getRepositories()
	if err != nil || len(repos) == 0 {
		prompt.PrintErrorf("No project found")
	}

	funk.ForEach(repos, func(repo gitRepository) {
		prompt.PrintItem(repo.NameWithNamespace + " ( " + strings.ReplaceAll(repo.Description, "\n", " ") + " )")
	})

	return nil
}

// Clone GIT repository matching with pattern
func (g *GitCommands) Clone(_ context.Context, c *cli.Command) error {

	var globStr = ""
	if c.NArg() > 0 {
		globStr = c.Args().Get(0)
	}

	_, error := g.cloneRepos(globStr, c.StringSlice("exclude"), true)
	return error
}

// PULL + REBASE all local GIT repository
func (g *GitCommands) Up(_ context.Context, c *cli.Command) error {

	folders, err := getGitFolders(c.Args().Get(0), c.StringSlice("exclude"))
	if err != nil {
		return err
	}

	funk.ForEach(folders, func(folder string) {

		args := g.authorization()
		args = append(args, []string{"-C", file.Rel(folder), "pull", "-q", "--rebase"}...)
		if c.Bool("stash") {
			args = append(args, "--autostash")
		}

		if out, err := cmd.ExecCmd("git", args); err != nil {
			prompt.PrintWarn("Cannot update '%s' ( %s )", folder, cmd.ErrorString(out))
			return
		} else {
			prompt.PrintInfo("'%s' %supdated", folder, prompt.Color(prompt.FgBlue))
		}
	})

	return nil
}

// Display status for all local GIT repository
func (g *GitCommands) St(_ context.Context, c *cli.Command) error {

	folders, err := getGitFolders(c.Args().Get(0), c.StringArgs("exclude"))
	if err != nil {
		return err
	}

	funk.ForEach(folders, g.status)

	return nil
}

// Add new ( group / orga / project ) on remote hosting
func (g *GitCommands) AddGroup(_ context.Context, c *cli.Command) error {

	_, err := g.impl.createGroup(c.Args())
	if err != nil {
		prompt.PrintErrorf("Cannot create group, error : %s", err.Error())
	}

	return nil
}

// Add new GIT repository on remote hosting
func (g *GitCommands) Add(_ context.Context, c *cli.Command) error {

	id, err := g.impl.createRepository(c.Args())
	if err != nil {
		return err
	}

	// Initialize
	pType := prompt.Input("\nEnter your project initializr type (empty|service|library|...) :", c.String("type"))
	if pType != "" && pType != "empty" {
		prompt.PrintNewLine()

		pName := prompt.Input("\nEnter your project initializr name :", c.String("name"))
		if pName == "" {
			prompt.PrintError("Name is required for initialize")
			return nil
		}

		prompt.PrintNewLine()
		pDeps := prompt.Input("\nEnter your project initializr dependencies ( separated with comma ) :", c.String("dependencies"))

		// Clone new repository
		repos, err := g.impl.getRepositories()
		if err != nil || len(repos) == 0 {
			prompt.PrintError("No project found")
			return err
		}

		repo := funk.Find(repos, func(repo gitRepository) bool { return repo.Id == id }).(gitRepository)

		_, cloneErr := g.clone(g.computeCloneUrl(repo), repo.Path)
		if cloneErr != nil {
			return cloneErr
		}

		// Init repo
		initErr := g.initRepo(repo.Path, pType, pName, pDeps)
		if initErr != nil {
			prompt.PrintErrorf("Cannot init repository, error : %s", err.Error())
		}
	}

	return nil
}

// Initialize existing GIT repository
func (g *GitCommands) Init(_ context.Context, c *cli.Command) error {

	// Init repository
	err := g.initRepo(c.Args().Get(0), c.Args().Get(1), c.Args().Get(2), c.Args().Get(3))
	if err != nil {
		prompt.PrintErrorf("Cannot init repository, error : %s", err.Error())
	}

	return nil
}

// Execute scripts on repositories
func (g *GitCommands) Exec(_ context.Context, c *cli.Command) error {

	pGlob := prompt.Input("\nEnter your project filter :", c.Args().Get(0))
	pAction := prompt.Input("\nEnter your action name :", c.Args().Get(1))

	acceptedInputs := []string{"y", "n", "a", "q"}
	acceptAll := false
	isInteractive := c.Bool("interactive")
	branchName := c.String("branch")
	commitMessage := c.String("commit-message")

	review := c.Bool("review")
	reviewTitle := c.String("review-title")
	if len(reviewTitle) == 0 {
		reviewTitle = commitMessage
	}
	reviewMessage := c.String("review-message")
	reviewDraft := c.Bool("review-draft")

	pParams := []string{}
	if c.Args().Len() < 2 {
		pParams = strings.Split(prompt.Input("\nEnter your action parameters :"), " ")
	} else {
		pParams = c.Args().Slice()[2:]
	}

	// Start by cloning matching repositories
	repos, err := g.cloneRepos(pGlob, c.StringArgs("exclude"), false)
	if err != nil {
		prompt.PrintErrorf("Cannot clone repositories (%s)", err.Error())
		return err
	}

	pathToRepo := funk.Map(repos, func(r gitRepository) (string, gitRepository) { return r.Path, r }).(map[string]gitRepository)

	folders, _ := getGitFolders(pGlob, c.StringArgs("exclude"))
	funk.ForEach(folders, func(folder string) {

		repo := pathToRepo[folder]

		defaultBranch := getBranch(folder)
		if repo.Id != "" {
			defaultBranch = repo.DefaultBranch
		}

		prompt.PrintInfo("Executing action for '%s'", folder)

		// Cleanup ( defaultBranch )
		prompt.PrintInfo("Executing action for '%s' : %sinitializing", folder, prompt.Color(prompt.FgYellow))
		g.cleanup(folder, defaultBranch)

		// Check branch if required
		if len(branchName) > 0 {
			branch(folder, branchName)
		}

		if isInteractive && !acceptAll {
			response := prompt.RestrictedInput("Initialization done, continue to execute?", acceptedInputs)
			switch response {
			case "q":
				os.Exit(0)
			case "n":
				return
			case "a":
				acceptAll = true
			}
		}

		// Execute action
		prompt.PrintInfo("Executing action for '%s' : %sexecuting", folder, prompt.Color(prompt.FgYellow))
		out, err := g.exec(folder, pAction, pParams)
		if err != nil {
			prompt.PrintErrorf("Cannot execute action for %s , error : %s \n%s", folder, err.Error(), out)
			return
		}

		if checkUncachedUncommitted(folder) {
			prompt.PrintInfo("Executing action for '%s' : %snothing to commit", folder, prompt.Color(prompt.FgYellow))
		} else if len(commitMessage) > 0 {

			if isInteractive && !acceptAll {

				for {
					response := prompt.RestrictedInput("Execution done, continue to commit?", append(acceptedInputs, "d"))
					if response == "y" {
						break
					} else if response == "q" {
						os.Exit(0)
					} else if response == "n" {
						return
					} else if response == "a" {
						acceptAll = true
						break
					} else if response == "d" {
						err := displayDiff(folder)
						if err != nil {
							prompt.PrintErrorf("Cannot execute diff for %s , error : %s \n", folder, err.Error())
						}
					}
				}
			}

			prompt.PrintInfo("Executing action for '%s' : %scommitting", folder, prompt.Color(prompt.FgYellow))
			addAndCommit(folder, commitMessage, true)

			if isInteractive && !acceptAll {
				response := prompt.RestrictedInput("Commit done, continue to push?", acceptedInputs)
				switch response {
				case "q":
					os.Exit(0)
				case "n":
					return
				case "a":
					acceptAll = true
				}
			}

			prompt.PrintInfo("Executing action for '%s' : %spushing", folder, prompt.Color(prompt.FgYellow))
			g.push(folder, branchName)

			if repo.Id != "" && len(branchName) > 0 && branchName != defaultBranch && review && len(reviewTitle) > 0 {

				if isInteractive && !acceptAll {
					response := prompt.RestrictedInput("Push done, continue to create "+g.GetLabels().CodeReviewRequest+"?", acceptedInputs)

					switch response {
					case "q":
						os.Exit(0)
					case "n":
						return
					case "a":
						acceptAll = true
					}
				}

				prompt.PrintInfo("Executing action for '%s' : %screating %s", folder, prompt.Color(prompt.FgYellow), g.GetLabels().CodeReviewRequest)
				review, err := g.impl.createReviewRequest(&repo, branchName, defaultBranch, reviewTitle, reviewMessage, reviewDraft)
				if err != nil {
					prompt.PrintErrorf("Cannot create %s for '%s' : , error : %s", g.GetLabels().CodeReviewRequest, folder, err.Error())
				} else {
					prompt.PrintInfo("Succeeded to create %s for '%s' , URL: %s%s", g.GetLabels().CodeReviewRequest, folder, prompt.Color(prompt.FgBlue), review.Url)
				}
			}

		}

		// Return to default branch
		if len(branchName) > 0 {
			checkout(folder, defaultBranch)
		}

		prompt.PrintInfo("Executed action for '%s' with %ssuccess", folder, prompt.Color(prompt.FgGreen))

	})

	return nil
}

// Execute scripts one repository
func (g *GitCommands) exec(folder string, action string, params []string) (string, error) {
	dir, _ := os.Getwd()
	return cmd.ExecCmdDir(dir+"/.microbox/actions/"+action, params, dir+"/"+folder)
}

// Clone GIT repository matching with pattern
func (g *GitCommands) cloneRepos(globStr string, exclusion []string, output bool) ([]gitRepository, error) {
	repos, err := g.impl.getRepositories()
	if err != nil || len(repos) == 0 {
		if output {
			prompt.PrintErrorf("No project found")
		}
	}

	var matcher = glob.NewGlobMatcher(globStr, exclusion...)

	funk.ForEach(repos, func(repo gitRepository) {

		match, reason := matcher.Match(repo.Path)
		if !match {
			if output {
				prompt.PrintInfo("%s %sskipping", reason, prompt.Color(prompt.FgYellow))
			}
			return
		}

		if file.Exist(repo.Path) {
			if output {
				prompt.PrintInfo("'%s' already existing, %sskipping", repo.Name, prompt.Color(prompt.FgBlue))
			}
		} else {

			if output {
				prompt.PrintInfo("'%s' not existing, %scloning%s into '%s'", repo.Name, prompt.Color(prompt.FgGreen), prompt.Color(prompt.FgWhite), repo.Path)
			}

			if out, err := g.clone(g.computeCloneUrl(repo), repo.Path); err != nil {
				prompt.PrintErrorf("Cannot clone ( %s )", err)
			} else {
				if checkIsRepoEmptyErr(out) {
					if output {
						prompt.PrintWarn("Repository is empty")
					}
				}
			}
		}
	})

	return repos, nil
}

// Initialize existing GIT repository
func (g *GitCommands) initRepo(folder string, projectType string, projectName string, deps string) error {

	// Initialize
	err := g.initializr.Init(projectType, projectName, strings.Split(deps, ","), file.Rel(folder))
	if err != nil {
		return err
	}

	// Commit
	addAndCommit(folder, "Initial commit", false)

	// Push
	g.push(folder, "")

	return nil
}

// Display status for local GIT repository
func (g *GitCommands) status(folder string) {

	repoUrl := getRepoUrl(folder)

	currentBranch := fmt.Sprintf("%.25s", getBranch(folder))

	var localInfo string
	if repoUrl == "" {
		localInfo = fmt.Sprintf(" %s[Local only repository]", prompt.Color(prompt.FgBlue))
	}

	if !checkUncachedUncommitted(folder) {
		prompt.PrintInfo("'%s' -> %s %sDirty (Uncached changes)%s", folder, currentBranch, prompt.Color(prompt.FgYellow), localInfo)
	} else if !checkCachedUncommitted(folder) {
		prompt.PrintInfo("'%s' -> %s %sDirty (Uncommitted changes)%s", folder, currentBranch, prompt.Color(prompt.FgYellow), localInfo)
	} else if !checkUntracked(folder) {
		prompt.PrintInfo("'%s' -> %s %sDirty (Untracked changes)%s", folder, currentBranch, prompt.Color(prompt.FgYellow), localInfo)
	} else if repoUrl == "" {
		/*
			If the "origin" URL is not defined in the project list, then no need
			to check for synchronization. It is clean if there is no untracked,
			uncached or uncommitted changes.
		*/
		prompt.PrintInfo("'%s' -> %s %sClean %s", folder, currentBranch, prompt.Color(prompt.FgGreen), localInfo)
	} else {

		// Fetch from remote
		g.fetch(folder)

		// Check for diverged branches
		local, remote := checkBranchOrigin(folder, currentBranch)

		if remote == "" {
			prompt.PrintInfo("'%s' -> %s %sNo remote branch", folder, currentBranch, prompt.Color(prompt.FgYellow))
		} else if local == "" {
			prompt.PrintInfo("'%s' -> %s %sInternal error", folder, currentBranch, prompt.Color(prompt.FgRed))
		} else if local != remote {
			prompt.PrintInfo("'%s' -> %s %sNot in sync", folder, currentBranch, prompt.Color(prompt.FgYellow))
		} else if local == remote {
			prompt.PrintInfo("'%s' -> %s %sClean", folder, currentBranch, prompt.Color(prompt.FgGreen))
		}
	}
}

// Compute Clone URL based on selected protocol
func (g *GitCommands) computeCloneUrl(repo gitRepository) string {
	cloneProtocol := g.config.Git.CloneProtocol
	switch cloneProtocol {
	case "ssh":
		return repo.SshUrl
	case "https":
		return repo.HttpUrl
	default:
		prompt.PrintErrorf("Invalid clone URL protocol '%s'", cloneProtocol)
		os.Exit(1)
		return ""
	}
}

// Compute authorization GIT parameter if required
func (g *GitCommands) authorization() []string {

	cloneProtocol := g.config.Git.CloneProtocol
	usePatToken := g.config.Git.UseTokenForOperation

	if cloneProtocol != "https" || !usePatToken {
		return []string{}
	}

	pat := base64.StdEncoding.EncodeToString([]byte(":" + g.config.Git.PrivateToken))
	return []string{"-c", "http.extraHeader=Authorization: Basic " + pat}
}

// Return all git folders matching with glob
func getGitFolders(globPattern string, exclusion []string) (folders []string, err error) {

	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	var matcher = glob.NewGlobMatcher(globPattern, exclusion...)

	var gitFolders []string

	walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {

			relativePath := strings.TrimPrefix(filepath.Clean(strings.TrimPrefix(path, dir)), string(os.PathSeparator))

			if strings.HasSuffix(relativePath, "/.git") {
				folder := strings.TrimSuffix(relativePath, "/.git")
				match, _ := matcher.Match(folder)
				if match {
					gitFolders = append(gitFolders, folder)
				}
			}

			if strings.Count(relativePath, string(os.PathSeparator)) >= MAX_DEPTH {
				return filepath.SkipDir
			}

			return nil
		}
		return nil
	})

	if walkErr != nil {
		return nil, walkErr
	}

	sort.Strings(gitFolders)

	return gitFolders, nil
}

// Return remote origin URL
func getRepoUrl(folder string) string {
	out, err := cmd.ExecCmd("git", []string{"-C", file.Rel(folder), "remote", "get-url", "origin"})
	if err != nil {
		return ""
	}

	return strings.TrimSpace(out)
}

// Return current branch name
func getBranch(folder string) string {
	out, _ := cmd.ExecCmd("git", []string{"-C", file.Rel(folder), "rev-parse", "--abbrev-ref", "HEAD"})
	return strings.Trim(out, "\n ")
}

// Return true if files is changed and not in GIT repo
func checkUncachedUncommitted(folder string) bool {
	_, err := cmd.ExecCmd("git", []string{"-C", file.Rel(folder), "diff", "--exit-code"})
	return err == nil
}

// Return true if files is changed and cached in GIT repo
func checkCachedUncommitted(folder string) bool {
	_, err := cmd.ExecCmd("git", []string{"-C", file.Rel(folder), "diff", "--cached", "--exit-code"})
	return err == nil
}

// Return true if files is changed and not tracked in GIT repo
func checkUntracked(folder string) bool {
	out, _ := cmd.ExecCmd("git", []string{"-C", file.Rel(folder), "status", "--porcelain"})
	return strings.Count(out, "?? ") == 0
}

// Check if a local branch points to the same commit as a remote branch
func checkBranchOrigin(folder string, branch string) (string, string) {
	local, localErr := cmd.ExecCmd("git", []string{"-C", file.Rel(folder), "rev-parse", "--verify", branch})
	remote, remoteErr := cmd.ExecCmd("git", []string{"-C", file.Rel(folder), "rev-parse", "--verify", "origin/" + branch})

	if localErr == nil && remoteErr == nil {
		return local, remote
	} else if localErr == nil {
		return local, ""
	} else if remoteErr == nil {
		return "", remote
	} else {
		return "", ""
	}
}

// Return true if files is changed and not tracked in GIT repo
func (g *GitCommands) clone(cloneUrl string, folder string) (string, error) {
	return cmd.ExecCmd("git", append(g.authorization(), []string{"clone", "-q", cloneUrl, folder}...))
}

// Return true if files is changed and not tracked in GIT repo
func (g *GitCommands) fetch(folder string) {
	cmd.ExecCmd("git", append(g.authorization(), []string{"-C", file.Rel(folder), "fetch"}...)) //nolint:errcheck
}

// Checkout existing branch
func checkout(folder string, branch string) {
	cmd.ExecCmd("git", []string{"-C", file.Rel(folder), "checkout", branch}) //nolint:errcheck
}

// Create new branch
func branch(folder string, branch string) {
	cmd.ExecCmd("git", []string{"-C", file.Rel(folder), "checkout", "-B", branch}) //nolint:errcheck
}

// Cleanup repo and update to specified branch
func (g *GitCommands) cleanup(folder string, branch string) {
	cmd.ExecCmd("git", []string{"-C", file.Rel(folder), "stash", "save", "Auto-stash by microcli"})              //nolint:errcheck
	cmd.ExecCmd("git", []string{"-C", file.Rel(folder), "checkout", branch})                                     //nolint:errcheck
	cmd.ExecCmd("git", append(g.authorization(), []string{"-C", file.Rel(folder), "pull", "-q", "--rebase"}...)) //nolint:errcheck
}

// Show diff
func displayDiff(folder string) error {
	return cmd.ExecAndOutCmd("git", []string{"-C", file.Rel(folder), "diff"})
}

// Return true if files is changed and not tracked in GIT repo
func addAndCommit(folder string, comment string, onlyTracked bool) {

	params := []string{"-C", file.Rel(folder), "add"}
	if onlyTracked {
		params = append(params, "-u")
	}

	out, err := cmd.ExecCmd("git", append(params, "."))
	if err != nil {
		prompt.PrintError(out)
		return
	}

	out, err = cmd.ExecCmd("git", []string{"-C", file.Rel(folder), "commit", "-m", comment})
	if err != nil {
		prompt.PrintError(out)
	}
}

// Return true if files is changed and not tracked in GIT repo
func (g *GitCommands) push(folder string, track string) {

	params := g.authorization()
	params = append(params, "-C", file.Rel(folder), "push")

	if len(track) > 0 {
		params = append(params, "-u", "origin", track)
	}

	out, err := cmd.ExecCmd("git", params)
	if err != nil {
		prompt.PrintError(out)
	}
}

// Check out string match with empty cloned repository
func checkIsRepoEmptyErr(output string) bool {
	return output == "warning: You appear to have cloned an empty repository.\n"
}

// Return GIT type implementation
func getImpl(config config.Config) gitRemote {

	impl := getImplCfg(config.Git.Type)
	if impl == nil {
		prompt.PrintErrorf("Invalid server type '%s,'", config.Git.Type)
		os.Exit(1)
	}

	return impl.Impl(config)
}

func getImplCfg(gitType string) *GitImplement {

	for e := range gitImplements {
		if strings.EqualFold(gitImplements[e].Id, gitType) {
			return &gitImplements[e]
		}
	}

	return nil

}
