# Design

<!-- impeccable:design-schema 1 -->

## Mode

Operate. The GUI is a local operator tool; trust, scanability, and standard desktop form behavior outrank decoration.

## Visual World

Dark, cool instrument surface with one mint-green action state. The interface is a single workbench: fixed navigation rail, compact top status, grouped configuration panels, sticky action bar, result table, and append-only log stream. The visual identity comes from restrained contrast, precise spacing, and state color, not imagery or a decorative theme.

## Tokens

- Background: `#0c1018`; panel `#121925`; raised panel `#172132`.
- Foreground: `#edf3ff`; muted `#8e9db5`; faint `#5e6c84`.
- Accent: `#6de2b1`; strong accent `#31c48d`; info `#7c9dff`; warn `#f5c76a`; danger `#ff7e8a`.
- Border: `#263349`; soft border `#1d293b`; focus halo `rgba(109, 226, 177, .35)`.
- Radius: 14px panels, 9px controls, 10px cards.
- Type: Segoe UI / Microsoft YaHei / PingFang SC system stack; fixed UI scale.

## Layout

- Desktop shell: 248px sidebar plus flexible main column.
- Configuration uses two equal columns; broad panels span both.
- At widths below 1100px, parameter grids collapse to two columns and the action bar stacks.
- At widths below 760px, the sidebar hides and all configuration columns become one column.

## Component Vocabulary

- Panels carry one purpose each, with a small technical tag on the right.
- Fields are labeled above controls, use 9px radii, and share hover/focus/disabled semantics.
- Primary action is mint only when enabled; all other buttons share the same neutral secondary shape.
- Status uses one pill, one progress track, and semantic text; no decorative metrics or hero numbers.
- Results use a compact data table with tabular identifiers.
- Logs use time / level / message rows with monospace message text.

## Motion

150–220ms state transitions on hover, focus, toast, and progress. No page-load choreography.

## States

Idle, running, success, failed, disabled, focus, hover, empty results, empty logs, invalid proxy hint, and missing runtime dependencies are all visible in the interface.

## Accessibility

Visible focus, semantic labels and headings, keyboard-operable controls, sufficient contrast, scrollable log and result regions, and no reliance on color alone for success/failure.
