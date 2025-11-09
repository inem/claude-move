# Claude Move

Interactive session picker for Claude Code. Find and resume your conversations in any directory.

## Problem

Working on a project in `/home/project` but want to continue that conversation from `/new/location`?

Claude Code sessions are tied to directories. This tool helps you find session IDs and generate the command to resume them anywhere.

## How It Works

Claude Code has `--resume <session-id>` flag. This tool:
1. Shows all sessions from current (or specified) directory
2. Interactive selection with ↑/↓ arrows
3. Shows conversation context (first & last messages)
4. Generates resume command for new location
5. Copies to clipboard

## Installation

```bash
make install
```

This installs `claude-move` to `~/go/bin/`.

## Usage

### Interactive Mode

```bash
claude-move
```

The tool will:
1. Find sessions from current directory
2. Show interactive list with ↑/↓ selection
3. Display conversation context
4. Ask for target directory
5. Generate & copy resume command

### From Specific Directory

```bash
claude-move --from /path/to/old/project
```

## Example

```bash
$ cd ~/old-project
$ claude-move
```

Output:
```
╔══════════════════════════════════════════╗
║   Claude Code Session Picker             ║
╚══════════════════════════════════════════╝

ℹ Looking for sessions in: /Users/inem/old-project

✓ Found 2 session(s)

Select session (↑/↓ arrows, Enter to confirm)

> [1] be0871b4...505ffa | 10 msgs | 2025-11-09 07:43 → 18:46
    Start: [Pasted text +56 lines] → Let's create a tool → How does this work?
    Last:  Fixed the issue → Testing now → All working!

  [2] bd0c34fc...6bcceb | 210 msgs | 2025-11-08 11:59 → 17:58
    Start: code quality check?
    Last:  Everything looks good

Enter directory to continue session: ~/new-project

╭──────────── Session Info ────────────╮
│ Session ID: be0871b4-ad1c-444d...    │
│ Messages:   10                       │
│ Started:    2025-11-09 07:43         │
│ Last:       2025-11-09 18:46         │
╰──────────────────────────────────────╯

╭────────── Resume Command ────────────╮
│ Run this:                            │
│                                      │
│   cd ~/new-project && claude --resume be0871b4... │
╰──────────────────────────────────────╯

Copy to clipboard? (y/N): y
✓ Copied! Just paste and run.
```

## Development

```bash
make build      # Build binary
make run        # Run locally
make test       # Test with example
make clean      # Clean up
```

## How Sessions Work

Claude Code stores session metadata in `~/.claude/history.jsonl`:

```json
{
  "project": "/Users/inem/my-project",
  "sessionId": "be0871b4-ad1c-444d-8279-d85f75505ffa",
  "display": "last message",
  "timestamp": 1762708392775
}
```

This tool reads `history.jsonl`, groups by `sessionId`, and lets you pick which conversation to resume using:

```bash
claude --resume <session-id>
```

The session ID works **from any directory** - you don't need to migrate files!

## Troubleshooting

**No sessions found**
- Make sure you're in the right directory (or use `--from`)
- Check `~/.claude/history.jsonl` exists

**Resume doesn't work**
- Make sure you're using Claude Code CLI (not web version)
- Try: `claude --help` to verify `--resume` flag exists

## License

MIT
