// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors
//
// netleash-spawn <program> [arguments...]
//
// Starts <program> as its OWN "responsible process" and waits for it. macOS attributes privacy prompts (Downloads,
// Documents, screen recording, ...) to the responsible process, which is by default the app that started the chain. A
// process started with this tool is identified by its own executable, so the protected app shows up under its own name.
// It runs as the user (netleash-exec has already dropped root and set the confined group), is not setuid, and only
// forwards signals and the exit status.
#include <dlfcn.h>
#include <errno.h>
#include <signal.h>
#include <spawn.h>
#include <stdio.h>
#include <string.h>
#include <sys/wait.h>
#include <unistd.h>

extern char **environ;
static volatile pid_t child = 0;

static void forward(int sig) {
    if (child > 0) kill(child, sig);
}

int main(int argc, char **argv) {
    if (argc < 2) {
        fprintf(stderr, "usage: netleash-spawn <program> [arguments...]\n");
        return 64;
    }
    posix_spawnattr_t attr;
    posix_spawnattr_init(&attr);
    typedef int (*disclaim_fn)(posix_spawnattr_t *, int);
    disclaim_fn disclaim = (disclaim_fn)dlsym(RTLD_DEFAULT, "responsibility_spawnattrs_setdisclaim");
    if (disclaim) disclaim(&attr, 1);

    pid_t pid;
    int rc = posix_spawn(&pid, argv[1], NULL, &attr, argv + 1, environ);
    if (rc != 0) {
        fprintf(stderr, "netleash-spawn: %s: %s\n", argv[1], strerror(rc));
        return 127;
    }
    child = pid;
    int sigs[] = {SIGINT, SIGTERM, SIGHUP, SIGQUIT, SIGUSR1, SIGUSR2};
    for (unsigned i = 0; i < sizeof sigs / sizeof *sigs; i++) signal(sigs[i], forward);

    int status = 0;
    while (waitpid(pid, &status, 0) < 0) {
        if (errno != EINTR) return 1;
    }
    if (WIFEXITED(status)) return WEXITSTATUS(status);
    if (WIFSIGNALED(status)) return 128 + WTERMSIG(status);
    return 1;
}
