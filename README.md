# NTA Camp Printables Generator

PDF generator for [404.camp](https://404.notrollsallowed.com) conference
printables.

## Setup

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

## Commands

```bash
./generate.py
usage: generate.py [-h] {labels,times,discord,signup} ...

positional arguments:
  {labels,times,discord,signup}
    labels              Generate presentation labels
    times               Generate time-slot labels
    discord             Generate Discord invite poster
    signup              Generate signup/registration list poster
```

See `examples/` directory for examples.
