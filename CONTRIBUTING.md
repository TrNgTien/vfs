# Contributing to vfs

First off, thank you for considering contributing to `vfs`! It's people like you that make `vfs` such a great tool for the AI coding community.

## How Can I Contribute?

### Add Your Use Case

If your project or company is using `vfs`, we'd love to feature you in our [README.md](README.md#used-by).

1. Fork this repo.
2. Navigate to the `Used By` section in `README.md`.
3. Add a row to the table with:
    - **Logo**: URL to your logo (or placeholder).
    - **Name**: Your company or project name.
    - **Website**: Your official website.
    - **Use Case**: A short description of how you use `vfs`.
    - **Blog**: (Optional) A link to a blog post or case study.
4. Submit a PR titled `usecase: add <Company/Project>`.

### Report Bugs

If you find a bug, please create an issue with:
- The `vfs` version (`vfs --version`).
- Your OS and Go version.
- A minimal reproducible example (e.g., a specific code file that fails to parse).

### Propose Features

We are always looking to improve `vfs`! If you have an idea for a new feature or language support, please open an issue first to discuss it.

### Contribute Code

1. Fork the repo and create your branch from `main`.
2. Ensure you have Go 1.24+ and a C compiler installed.
3. Run `make test` (or `go test ./...`) to ensure everything is working.
4. Make your changes.
5. If you add a new language, update `AGENTS.md` and `README.md` supported languages tables.
6. Submit your PR!

## Development Setup

`vfs` uses `go` for its core logic and `tree-sitter` for parsing.

```bash
git clone https://github.com/TrNgTien/vfs.git
cd vfs
go mod download
npx tree-sitter generate  # if you're modifying parsers
make build
```

## Community & Support

- **Discussions**: Use GitHub Discussions for questions or ideas.
- **Rules**: Be kind and respectful to all contributors.

---

*“Less tokens, more context.”*
