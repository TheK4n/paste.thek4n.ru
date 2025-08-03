# Contributing guide
We appreciate your interest in contributing to our project! Please take a
moment to review this guide to ensure a smooth collaboration.


## Found a bug?
If you find a bug in the source code, you can help us by submitting an issue to
our GitHub Repository. Even better, you can submit a Pull Request with a fix.


### Submitting a Pull Request (PR)
Before you submit your Pull Request (PR) consider the following guidelines:
1. Search GitHub for an open or closed PR that relates to your submission. You
   don't want to duplicate effort.

2. Fork this repo

3. Make your changes in a new git branch with tests:
```sh
git checkout -b my-fix-branch master
```

4. Run tests, and ensure that all tests pass.
```sh
make test
```

5. Its recommended to set up [Pre-commit hooks](#pre-commit)

6. Commit your changes with a meaningful message [Commit message format](#commit-message-format)

7. Push your branch to GitHub:
```sh
git push origin my-fix-branch
```

8. In Github, send a PR to master


## <a name="rules"></a> Coding Rules
To ensure consistency throughout the source code, keep these rules in mind as
you are working:

* All features or bug fixes must be tested by one or more specs.
* All public API methods must be documented.

### <a name="commit-message-format"></a> Commit message format
We use [this](https://www.conventionalcommits.org/en/v1.0.0/) standard
```gitcommit
<type>(<scope>): <subject>
<BLANK LINE>
[optional body]
<BLANK LINE>
[optional footer]
```

### <a name="pre-commit"></a> Setting Up Pre-Commit Hooks
We use pre-commit to automatically enforce code quality checks before commits:
```sh
python3 -m venv venv
. ./venv/bin/activate
pip install pre-commit
pre-commit install
```
Hooks will now run automatically on every commit.
