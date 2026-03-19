# skill-manager

[![PyPI](https://img.shields.io/pypi/v/skill-manager.svg)](https://pypi.org/project/skill-manager/)
[![Changelog](https://img.shields.io/github/v/release/NickCellino/skill-manager?include_prereleases&label=changelog)](https://github.com/NickCellino/skill-manager/releases)
[![Tests](https://github.com/NickCellino/skill-manager/actions/workflows/test.yml/badge.svg)](https://github.com/NickCellino/skill-manager/actions/workflows/test.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://github.com/NickCellino/skill-manager/blob/master/LICENSE)

CLI tool to manage coding agent skills

## Installation

Install this tool using `uv`:
```bash
uv pip install skill-manager
```
## Usage

For help, run:
```bash
skill-manager --help
```
You can also use:
```bash
python -m skill_manager --help
```
## Development

To contribute to this tool, first checkout the code. Then create a new virtual environment:
```bash
cd skill-manager
uv venv
source .venv/bin/activate
```
Now install the dependencies and test dependencies:
```bash
uv pip install -e '.[test]'
```
To run the tests:
```bash
uv run pytest
```

