---
title: gh-claude
template: home.html
hide:
  - navigation
  - toc
---

<!--
  The home page is rendered entirely by overrides/home.html (the landing).
  This body only provides the page title and meta description for the <head>.
-->

A GitHub CLI extension that launches Claude Code with a temporary,
least-privilege GitHub token — read-only on source code (no push), read/write on
issues and pull requests, expiring after 7 days, and stored in your OS keychain
(1Password optional). Claude works your private repos without ever seeing your
real credential.
