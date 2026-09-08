# Paste Sanitization Guide

## Problem
When pasting text into the terminal UI, special characters like `[106;5u` (terminal escape sequences) appear, breaking readability and storage.

## Root Cause
Terminal applications in raw mode can receive pasted data that includes:
- Bracketed-paste mode wrapping (`ESC [200~` and `ESC [201~`)
- ANSI escape sequences (cursor positioning, colors, etc.)
- Control characters (tabs, carriage returns, etc.)
- Mixed line endings (`\r\n`, `\r`)

## Solution: Sanitize Paste Input

### Implementation
Sanitize all paste input before inserting into the prompt:

```go
// route.go - Route PasteMsg to handler
func (m *Model) route(message tea.Msg) tea.Cmd {
    case tea.PasteMsg:
        return m.handlePaste(message)
    // ... other cases
}

// model.go - Handle paste with sanitization
func (m *Model) handlePaste(msg tea.PasteMsg) tea.Cmd {
    clean := sanitizePaste(msg.Content)
    if clean == "" {
        return nil
    }
    m.prompt.InsertString(clean)
    m.refreshMenu()
    return nil
}
```

### Sanitization Logic

```go
func sanitizePaste(s string) string {
    // 1. Strip bracketed-paste mode wrappers
    s = strings.ReplaceAll(s, "\x1b[200~", "")
    s = strings.ReplaceAll(s, "\x1b[201~", "")
    
    // 2. Normalize line endings
    s = strings.ReplaceAll(s, "\r\n", "\n")
    s = strings.ReplaceAll(s, "\r", "\n")
    
    // 3. Strip ANSI/DCS sequences
    s = stripControlSequences(s)
    
    // 4. Replace tabs with spaces
    s = strings.ReplaceAll(s, "\t", " ")
    
    // 5. Remove remaining control chars (preserve newlines)
    return filterControls(s)
}
```

### What It Removes

1. **Bracketed paste wrappers**:
   - `\x1b[200~` - Paste start
   - `\x1b[201~` - Paste end

2. **ANSI escape sequences**:
   - CSI sequences: `ESC [` followed by parameters and final byte
   - OSC sequences: `ESC ] ... BEL` or `ESC ] ... ESC \`
   - DCS sequences: `ESC P ... ST`

3. **Control characters**:
   - Tabs (converted to spaces)
   - Remaining control characters (except newlines)

### What It Preserves

1. **Printable characters**: All regular text
2. **Unicode**: Emojis, accents, international characters
3. **Newlines**: For multi-line pasted content
4. **Spaces**: Normal whitespace

## Testing

The implementation includes comprehensive tests:

```bash
go test ./internal/tui/ -run Paste -v
```

Tests cover:
- Stripping bracketed-paste wrappers
- Normalizing Windows line endings (`\r\n` → `\n`)
- Removing embedded control sequences (e.g., cursor positioning)
- Preserving regular text with emojis/unicode
- Handling tabs and empty input

## Benefits

1. **Cleaner input**: No invisible escape sequences in stored prompts
2. **Consistent line endings**: Uniform newlines regardless of source
3. **Safer processing**: No control characters to break processing
4. **Better UX**: Pasted text appears as expected, not garbled
5. **Defensive coding**: Handles multiple terminal paste formats robustly

## Best Practices

1. **Process early**: Sanitize at the point of input reception
2. **Preserve intent**: Keep newlines for legitimate multi-line pastes
3. **Handle edge cases**: Empty input, only wrapping sequences
4. **Don't overcomplicate**: Focus on removing harmful sequences, not beautifying
5. **Test thoroughly**: Verify with various pasted content types