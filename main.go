package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/google/go-github/github"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
)

// Sortable Gitea repository list
type GTRepositoryList []*gitea.Repository

func (list GTRepositoryList) Len() int {
	return len(list)
}

func (list GTRepositoryList) Less(i, j int) bool {
	return list[i].Name < list[j].Name
}

func (list GTRepositoryList) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

// Sortable Github repository list
type GHRepositoryList []*github.Repository

func (list GHRepositoryList) Len() int {
	return len(list)
}

func (list GHRepositoryList) Less(i, j int) bool {
	return *list[i].Name < *list[j].Name
}

func (list GHRepositoryList) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

func initConfig() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s config.yaml\n", os.Args[0])
		os.Exit(1)
	}

	fd, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Printf("open: %s: %s\n", os.Args[0], err)
		os.Exit(1)
	}
	defer fd.Close()

	viper.SetConfigType("yaml")
	err = viper.ReadConfig(fd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, config := range []string{"GitHub.PersonalToken", "Gitea.ServerURL", "Gitea.PersonalToken"} {
		if viper.GetString(config) == "" {
			fmt.Printf("key %s is missing from configuration file\n", config)
			os.Exit(1)
		}
	}
}

func initLogFile() {
	logFile := viper.GetString("LogFile")
	if logFile != "" {
		logHandle, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Printf("Cannot open log file '%s': %s\n", logFile, err)
			os.Exit(1)
		}
		log.SetOutput(logHandle)
	}
}

func initGitHubClient() (*github.Client, *github.User) {
	ghToken := viper.GetString("GitHub.PersonalToken")
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: ghToken},
	)
	ghClient := github.NewClient(oauth2.NewClient(ctx, ts))

	ghUserInfo, _, err := ghClient.Users.Get(ctx, "")
	if err != nil {
		log.Fatal(err)
	}

	return ghClient, ghUserInfo
}

func listGitHubRepositories(ghClient *github.Client) GHRepositoryList {
	var allRepos []*github.Repository
	opt := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		repos, resp, err := ghClient.Repositories.List(context.Background(), "", opt)
		if err != nil {
			if _, ok := err.(*github.RateLimitError); ok {
				log.Printf("Reached the GitHub API rate limiting. Sleeping for a while...\n")
				time.Sleep(60 * time.Second)
				continue
			} else {
				log.Fatal(err)
			}
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allRepos
}

func filterGitHubRepositories(ghRepos GHRepositoryList, ghUsername string) GHRepositoryList {
	var filteredRepos []*github.Repository
	for _, repo := range ghRepos {
		// Mirror only repositories that the user owns and has not forked from someone else
		if !*repo.Fork && *repo.Owner.Login == ghUsername {
			filteredRepos = append(filteredRepos, repo)
		}
	}
	return filteredRepos
}

func initGiteaClient() (*gitea.Client, *gitea.User) {
	gtClient := gitea.NewClient(viper.GetString("Gitea.ServerURL"), viper.GetString("Gitea.PersonalToken"))
	gtUserInfo, err := gtClient.GetMyUserInfo()
	if err != nil {
		log.Fatal(err)
	}
	return gtClient, gtUserInfo
}

func listGiteaRepositories(gtClient *gitea.Client) GTRepositoryList {
	var allRepos GTRepositoryList

	opt := gitea.ListReposOptions{
		ListOptions: gitea.ListOptions{PageSize: 50},
	}
	var page int = 1
	for {
		opt.Page = page
		repos, err := gtClient.ListMyRepos(opt)
		if err != nil {
			log.Fatal(err)
		}
		allRepos = append(allRepos, repos...)

		if len(repos) == 0 || len(repos) != opt.ListOptions.PageSize {
			// End of repository list
			break
		}

		page = page + 1
	}

	return allRepos
}

func computeRepositoriesToMigrate(gh GHRepositoryList, gt GTRepositoryList) GHRepositoryList {
	var toMigrate GHRepositoryList = make(GHRepositoryList, 0)

	if len(gh) == 0 {
		return gh
	}
	if len(gt) == 0 {
		return gh
	}
	var ghi int = 0
	var gti int = 0
	for {
		if ghi >= len(gh) { // No more GitHub repos to process...
			if gti >= len(gt) { // and no more Gitea repos to process !
				break
			}

			// On Gitea but not anymore on GitHub. We could remove the repo from gitea
			// but to be safe, keep it there.
			log.Printf("Gitea Repository %s is missing from GitHub, leaving it there...\n", gt[gti].Name)
			gti = gti + 1
			continue
		}
		if gti >= len(gt) { // No more Gitea repos to process
			toMigrate = append(toMigrate, gh[ghi:len(gh)]...)
			break
		}

		if *gh[ghi].Name == gt[gti].Name {
			ghi = ghi + 1
			gti = gti + 1
		} else if *gh[ghi].Name < gt[gti].Name {
			// On GitHub but not yet on Gitea. Migrate it.
			toMigrate = append(toMigrate, gh[ghi])
			ghi = ghi + 1
		} else {
			// On Gitea but not anymore on GitHub. We could remove the repo from gitea
			// but to be safe, keep it there.
			log.Printf("Gitea Repository %s is missing from GitHub, leaving it there...\n", gt[gti].Name)
			gti = gti + 1
		}
	}

	return toMigrate
}

func migrate(ghRepo *github.Repository, gtClient *gitea.Client, gtUser *gitea.User) error {
	// the description is an optional field
	var description string = ""
	if ghRepo.Description != nil {
		description = *ghRepo.Description
	}

	migrationOptions := gitea.MigrateRepoOption{
		CloneAddr:    *ghRepo.CloneURL,
		AuthUsername: *ghRepo.Owner.Login,
		AuthPassword: viper.GetString("GitHub.PersonalToken"),
		UID:          int(gtUser.ID),
		RepoName:     *ghRepo.Name,
		Mirror:       true,
		Private:      *ghRepo.Private,
		Description:  description,
	}
	_, err := gtClient.MigrateRepo(migrationOptions)
	return err
}

func main() {
	initConfig()
	initLogFile()

	// List Gitea repositories
	gtClient, gtUserInfo := initGiteaClient()
	log.Printf("Connected to Gitea as %s.\n", gtUserInfo.UserName)
	gtRepos := listGiteaRepositories(gtClient)
	log.Printf("Found %d Gitea repositories\n", len(gtRepos))

	// List GitHub repositories
	ghClient, ghUserInfo := initGitHubClient()
	log.Printf("Connected to GitHub as %s.\n", *ghUserInfo.Login)
	ghRepos := listGitHubRepositories(ghClient)
	log.Printf("Found %d GitHub repositories.\n", len(ghRepos))
	ghRepos = filterGitHubRepositories(ghRepos, *ghUserInfo.Login)
	log.Printf("There are %d repositories left after filtering.\n", len(ghRepos))

	// Sort all lists by repository name so that computeRepositoriesToMigrate
	// can perform a diff
	sort.Sort(gtRepos)
	sort.Sort(ghRepos)
	toMigrate := computeRepositoriesToMigrate(ghRepos, gtRepos)
	log.Printf("There are %d repositories to migrate from GitHub to Gitea.\n", len(toMigrate))

	// Migrate each repository
	var ok bool = true
	for _, repo := range toMigrate {
		log.Printf("Migrating %s...\n", *repo.Name)
		err := migrate(repo, gtClient, gtUserInfo)
		if err != nil {
			log.Printf("MigrateRepo: %s: %s", *repo.Name, err)
			ok = false
		}
	}

	if !ok {
		os.Exit(1)
	}
}
