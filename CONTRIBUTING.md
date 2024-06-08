# Contributing
When contributing to this repository, please first discuss the change you wish to make via issue, email, or any other method with the owners of this repository before making a change.

Please note we have a code of conduct, please follow it in all your interactions with the project.

## Table of Contents

1. [Code of Conduct](#code-of-conduct)
2. [How to Contribute](#how-to-contribute)
    - [Reporting Bugs](#reporting-bugs)
    - [Suggesting Enhancements](#suggesting-enhancements)
    - [Submitting Pull Requests](#submitting-pull-requests)
3. [Development Setup](#development-setup)
4. [Style Guides](#style-guides)
    - [Git Commit Messages](#git-commit-messages)
    - [Go Code Style](#go-code-style)

## Code of Conduct

Please read and follow our [Code of Conduct](CODE_OF_CONDUCT.md) to keep our community approachable and respectful.

## How to Contribute

### Reporting Bugs

If you find a bug in the project, please check if the bug is already reported. If not, open a new issue and include the following details:
- A clear and descriptive title.
- A detailed description of the bug.
- Steps to reproduce the bug.
- Any relevant logs, error messages, or screenshots.

### Suggesting Enhancements

If you have an idea for an enhancement or a new feature, please open an issue and describe:
- The problem you’re trying to solve.
- The solution or enhancement you propose.
- Any alternatives you’ve considered.


### Submitting Pull Requests

The `develop` branch is used for ongoing development and should always be in sync with the `main` branch. All new features and bug fixes should be developed in feature branches created from `develop`, and pull requests should be submitted to the `develop` branch.

1. Fork the repository and create your branch from `develop`.
2. Ensure your code follows the project's coding standards.
3. If you’ve added or changed functionality, update the documentation accordingly.
4. Review and follow the [manual QA guide](docs/qa/dev-manual-qa-guide.md) to ensure all known use cases are checked before committing.
5. Commit your changes with a descriptive commit message.
6. Push your branch to your fork and open a Pull Request to the `develop` branch of the repository.

Please ensure your pull request adheres to the following guidelines:
- Provide a clear description of what your pull request does.
- Include the issue number if your pull request addresses a specific issue.
- Follow the [manual QA guide](docs/qa/dev-manual-qa-guide.md)
- Review the changes to make sure they are well-tested and documented.

The repository owner will:
1. Review and test the changes.
2. Merge the changes into the `main` branch after successful review and testing.
3. Build the image and push it to Docker Hub.
4. Update CHANGELOG.md
5. Set up the tag.

## Development Setup

To set up the project for development, please refer to the [development guide](docs/develop.md).

## Style Guides

### Git Commit Messages

- Use the present tense ("Add feature" not "Added feature").
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...").
- Limit the first line to 72 characters or less.
- Reference issues and pull requests liberally after the first line.

### Go Code Style

- Write clear, concise comments for exported functions and types.
- Use descriptive names for variables and functions.

Thank you for your contributions!
