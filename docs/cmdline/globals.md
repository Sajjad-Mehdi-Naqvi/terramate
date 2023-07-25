---
title: terramate globals - Command
description: With the terramate globals command you print all globals used in stacks recursively.

# prev:
#   text: 'Stacks'
#   link: '/stacks/'

# next:
#   text: 'Sharing Data'
#   link: '/data-sharing/'
---

# Globals

**Note:** This is an experimental command that is likely subject to change in the future.

The `globals` command outputs all globals computed for a stack and all child stacks recursively.

## Usage

`terramate experimental [options] globals`

## Examples

Print globals for the stack in the current directory:

```bash
terramate experimental globals
```

Change the working directory: 

```bash
terramate experimental globals --chdir stacks/example
```