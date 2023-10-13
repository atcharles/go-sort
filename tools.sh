#!/usr/bin/env bash
script_name=$(basename "$0")
git_init() {
    remote_url=$(git config --local --get remote.origin.url)
    set -vuex
    rm -rf ./.git && git init
    git checkout --orphan latest_branch
    git add -A .
    git commit -am "Initial commit"
    #if remote_url is not empty, then add remote url
    test -n "$remote_url" && git remote add origin "$remote_url"
    if test $# -eq 0 || test -z "$1" || test -z "$remote_url"; then
        git checkout -b main
        git branch -D latest_branch
        set +vuex
    fi
    if test "$1" = 'reset'; then
        echo "git reset"
        git checkout -b main
        git branch -D latest_branch
        git fetch
        git branch --set-upstream-to=origin/main main
        set +vuex
    fi
    if test "$1" = 'pull'; then
        echo "git pull"
        git fetch
        git checkout -b main origin/main
        git branch -D latest_branch
        git pull
        set +vuex
    fi
}
git_push() {
    git_init "reset"
    git push -f origin main
    git pull
}
git_add() {
    git add -A .
    git commit -am "update"
}
loopGoSort() {
    find ./*.go -type f | while read -r file; do echo "$file" && go-sort "$file"; done
}
run() {
    echo "$script_name"
    case $1 in
    git-init) shift 1 && git_init "$@" ;;
    git-reset) shift 1 && git_init "reset" ;;
    git-pull) shift 1 && git_init 'pull' ;;
    git-push) shift 1 && git_push "$@" ;;
    git-add) shift 1 && git_add "$@" ;;
    gosort) shift 1 && loopGoSort "$@" ;;
    *) echo "unknown command: $1" ;;
    esac
}
run "$@"
