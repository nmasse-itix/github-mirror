# GitHub Repository mirroring for Gitea

## Context

Gitea is a nice alternative to GitHub, easy to setup.
When using it as a mirror from GitHub, you might want all repositories created in GitHub to appear automatically in Gitea.

## Compilation

```sh
go build -o mirror-github
```

## Pre-requisites

* Generate a [GitHub Personal Access Token](https://github.com/settings/tokens)
* Generate a Gitea Access Token from `https://GITEA_SERVER/user/settings/applications`

## Usage

```sh
cat > config.yaml <<EOF
GitHub:
  PersonalToken: GITHUB_PERSONAL_ACCESS_TOKEN
Gitea:
  ServerURL: https://GITEA_SERVER
  PersonalToken: GITEA_ACCESS_TOKEN
EOF
```

```sh
./github-mirror config.yaml
```

## In a crontab

In a crontab, you can add the `LogFile` directive to `config.yaml` to collect logs over multiple runs.
