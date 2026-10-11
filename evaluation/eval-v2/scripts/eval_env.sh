#!/usr/bin/env bash
# ==============================================================================
# eval_env.sh — Environment Protection Configuration for Evaluation Runs
# ==============================================================================
# Usage:
#   source evaluation/eval-v2/scripts/eval_env.sh
#
# Context & Rationale:
# During evaluation runs, coding agents operate autonomously on local checkouts.
# To guarantee reproducible isolation and prevent accidental leaks or mutations
# to remote repositories and GitHub, this file overrides default credentials
# and network auth helpers:
#
# 1. GH_TOKEN=invalid:
#    Disables GitHub CLI (`gh`) authentication. On macOS, `gh` normally retrieves
#    OAuth tokens stored in the system keychain with extensive permissions (repo,
#    workflow, delete_repo). The `gh` CLI gives precedence to the GH_TOKEN
#    environment variable over keychain entries. By setting GH_TOKEN to an
#    invalid string, any attempt by an agent or tool to invoke `gh` (e.g. creating
#    issues, comments, or opening pull requests) fails immediately with HTTP 401.
#
# 2. GIT_CONFIG_NOSYSTEM=1:
#    Tells Git to completely bypass system-level configuration files (/etc/gitconfig
#    and Xcode's embedded gitconfig). On macOS, Xcode provides a system config that
#    enables 'credential.helper = osxkeychain'. Setting GIT_CONFIG_NOSYSTEM=1
#    strips this helper from Git's configuration resolution, ensuring Git cannot
#    silently use stored keychain credentials to push over HTTPS to any remote.
#
# 3. GIT_TERMINAL_PROMPT=0:
#    When credentials are missing or invalid, Git defaults to interactively
#    prompting on the terminal for a username and password. In automated,
#    unattended, or scripted evaluation environments, this causes Git commands
#    to hang indefinitely. Setting GIT_TERMINAL_PROMPT=0 forces Git to fail
#    immediately closed with an error code rather than waiting for user input.
#
# 4. unset SSH_AUTH_SOCK:
#    Detaches the environment from any running macOS ssh-agent. While the primary
#    remotes use HTTPS, unsetting SSH_AUTH_SOCK ensures no fallback authentication
#    via local SSH keys can succeed if an agent attempts SSH operations.
# ==============================================================================

export GH_TOKEN=invalid
export GIT_CONFIG_NOSYSTEM=1
export GIT_TERMINAL_PROMPT=0
unset SSH_AUTH_SOCK
