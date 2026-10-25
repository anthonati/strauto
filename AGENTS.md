# Public repository boundary

This repository is public. Keep source code, tests, migrations, example configuration, and reusable documentation here.

Keep project planning, backlogs, session handoffs, work logs, deployment records, account/project identifiers, and security-sensitive information in the sibling `../strauto-private/` directory. Read its `README.md` and `PLAN.md` before continuing work when they are available. If that directory is absent, create private notes outside this checkout rather than adding them to the repository.

Store local credential values only in the private workspace's `secrets/` directory or the relevant service's secret store. Do not put them in this checkout, including ignored files, symlinks, generated assets, command output, or deployment archives. Public configuration examples must use empty values or obvious placeholders.

Keep public documentation deployment-neutral. Use example domains and describe behavior rather than recording live rollout status, infrastructure identifiers, account details, or future project plans. Keep private operational details out of commit messages, pull request descriptions, and issues.

When moving previously published notes, remember that deleting files does not erase earlier commits or other branches. Do not rewrite remote history or rotate working credentials without an explicit reason and the necessary authorization.
