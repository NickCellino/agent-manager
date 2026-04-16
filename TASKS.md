# Tasks

## Source PRD

`PRD.md`

## Task Checklist

- [x] Task 1: Visible Skills Window
- [x] Task 2: Page-Stride Navigation
- [ ] Task 3: Viewport Context Footer

## Task 1: Visible Skills Window

**Type**: AFK

**What to build**

Implement the foundational scrolling behavior for the skills picker described in the PRD's Solution and Implementation Decisions sections. This slice should make the skills list render only the visible rows, keep the selected skill visible during single-row navigation, and adapt correctly when the terminal size changes.

**Acceptance criteria**

- [x] The skills picker renders only the visible subset of filtered skills instead of the full list when the list exceeds the terminal height.
- [x] Single-row navigation with `j/k` and `up/down` keeps the selected skill visible by auto-scrolling as needed.
- [x] Resizing the terminal recomputes the visible window and preserves a valid selected row without introducing off-screen selection bugs.
- [x] Focused tests cover viewport math and cursor visibility at the top, middle, end, and short-list cases.

**Blocked by**

None - can start immediately

**User stories addressed**

- User story 1
- User story 2
- User story 3
- User story 5
- User story 12

## Task 2: Page-Stride Navigation

**Type**: AFK

**What to build**

Add page-stride keyboard navigation for the skills picker on top of the visible-window behavior from the PRD's Solution and Implementation Decisions sections. This slice should let users move by the current visible row count in navigate mode, while preserving filter mode as pure text input and keeping the help footer aligned with the actual controls.

**Acceptance criteria**

- [x] Pressing `h/l` moves the selected skill backward or forward by the current visible row count and clamps correctly at the list bounds.
- [x] Pressing `H/L` behaves the same as lowercase in navigate mode.
- [x] While the filter input is focused, `h` and `l` are treated as text input and do not trigger paging.
- [x] The navigate-mode help text documents the new page-stride controls.
- [x] Focused tests cover forward and backward page jumps, clamping behavior, and the navigate-vs-filter mode distinction.

**Blocked by**

Task 1: Visible Skills Window

**User stories addressed**

- User story 4
- User story 6
- User story 11
- User story 12

## Task 3: Viewport Context Footer

**Type**: AFK

**What to build**

Add footer context for the scrollable skills picker described in the PRD's Solution and Implementation Decisions sections. This slice should show a human-friendly visible-range indicator based on the filtered results while preserving the global selected-count summary and maintaining a stable empty-state footer.

**Acceptance criteria**

- [ ] The footer includes a 1-based inclusive visible-range line in the form `a-b of N shown` based on the filtered result set.
- [ ] The existing selection summary remains global as `selected/all skills selected`, even when a filter is active.
- [ ] When the filtered result set is empty, the UI still shows a stable range line such as `0 of 0 shown`.
- [ ] The visible-range values stay correct while navigating, filtering, and resizing.
- [ ] Focused tests cover unfiltered, filtered, and empty-result footer states.

**Blocked by**

Task 1: Visible Skills Window

**User stories addressed**

- User story 7
- User story 8
- User story 9
- User story 10
- User story 12
