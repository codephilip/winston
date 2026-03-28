# Winston Mobile App — Design Brief

## Overview

Design a native-feeling mobile app (iOS-first) for **Winston** — a personal AI agent platform that runs on your own machine and gives you access to a suite of specialized AI agents from anywhere. Think of it as your personal command centre: intelligent, always-on, and deeply connected to your digital life.

The app is primarily a **voice-first, text-capable conversational interface** — like ChatGPT's Advanced Voice Mode, but with the ability to switch between distinct agents and AI models mid-session. The experience should feel intimate, fast, and powerful. Not a productivity tool. A personal AI companion.

---

## Aesthetic Direction

### Mood
**Dark. Moody. Premium. Focused.**

This is not a bright SaaS product. It lives in the dark — like a terminal that grew up. The vibe sits between:

- **Claude** — warm, thoughtful, slightly editorial. Subtle gradients, generous whitespace, considered typography.
- **ChatGPT** — clean, structured, minimal chrome. Confident use of space, readable at a glance.
- **Neither** — push past both into something more expressive and personal. Less corporate. More like a high-end audio app or a dark-mode code editor that's been beautifully designed.

### References to draw from
- Claude.ai dark mode (warmth, subtlety)
- ChatGPT voice mode (the orb animation, the listening state, the simplicity)
- Linear app (micro-interactions, speed, dark precision)
- Raycast (command-palette energy, keyboard-first feel translated to touch)
- Teenage Engineering product UIs (muted, slightly industrial, distinct)

### Colour Palette
Start from this direction — refine it:

- **Background**: Near-black with a slight warm or cool tint — not pure `#000000`. Think `#0C0C0E` or `#0D0C10`.
- **Surface**: Subtle elevation — `#161618` for cards/panels, `#1E1E22` for active states.
- **Primary accent**: A muted but distinct colour used sparingly for the active agent. Each agent has its own identity colour (see Agent Identities below) — only the active one should bleed into the UI.
- **Text**: Off-white `#F2F0ED` for primary, `#8A8894` for secondary/meta, never pure white.
- **Voice state**: A signature glow/pulse that shifts based on what's happening (listening, thinking, speaking).
- **Borders**: Near-invisible — `1px` at 8% opacity. Used to separate, never to decorate.

### Typography
- One typeface throughout. Consider: **Inter**, **Geist**, or a slightly characterful option like **Söhne** or **DM Sans**.
- Hierarchy through weight and size, not colour.
- Message text: 16px / relaxed line-height. Comfortable for reading long AI responses.
- System labels / meta: 11–12px, tracked out, muted.

---

## Agent Identities

Each agent has a colour and a character. When an agent is active, its colour subtly influences the UI — the voice orb, the active indicator, small accent touches. These should feel like "moods", not loud brand colours.

| Agent       | Role                          | Colour Direction         |
|-------------|-------------------------------|--------------------------|
| Winston     | Personal orchestrator         | Amber / warm gold        |
| Marketing   | Competitive intel, content    | Electric blue            |
| Pentester   | Security research, recon      | Red / deep crimson       |
| YouTube     | Video production pipeline     | Violet / deep purple     |
| Designer    | UI design, Figma, frontend    | Emerald / cool green     |

These are not bright or saturated. They are **muted, deep versions** of each colour — the kind that read as a glow in a dark room, not as a label.

---

## Core Screens

### 1. Home / Agent Select

The entry point. This is where you choose who you're talking to.

- Full-screen dark canvas
- Agents displayed as a **scrollable horizontal strip** or a **radial/stacked layout** — not a boring grid list
- Each agent: name, one-line descriptor, its identity colour as a subtle ambient glow behind it
- The "recently used" or default agent is pre-selected / most prominent
- No unnecessary chrome. No hamburger menu. No bottom tab bar with 5 icons.
- Tapping an agent transitions directly into the conversation view with a smooth, contextual animation (the agent's colour blooms outward)

### 2. Conversation View — Text Mode

The main chat interface.

- Messages fill the screen. Nothing else competes.
- **User messages**: Right-aligned, slightly elevated surface, clean.
- **Agent messages**: Left-aligned, no bubble — the text breathes directly on the background. Use subtle left-border accent in the agent's colour optionally.
- **Streaming text**: Characters animate in naturally — not letter by letter, but in natural word-chunks with a subtle fade. Like the model is thinking and writing simultaneously.
- **Markdown rendering**: Code blocks, bold, headers all styled cleanly and readably. Code gets a monospace dark surface.
- At the bottom: a **minimal input bar** — text field + voice button. When idle it's just a single-line floating bar. When focused it expands slightly.
- The **voice button** is the most prominent element in the input bar — bigger than send, centered or right-weighted. It should feel like the primary affordance.
- Scroll behaviour: new messages auto-scroll. User can scroll up to read history without interruption.

### 3. Voice Mode — The Orb State

This is the centrepiece of the app. Think ChatGPT's voice orb, but more considered.

Triggered from the conversation view by holding or tapping the voice button.

**States to design:**

- **Idle / Ready to listen**: A subtle animated orb in the agent's colour. Slow, breathing pulse. Like something alive but waiting.
- **Listening**: The orb expands and reacts to the user's voice amplitude in real time — fluid, organic waveform or blob morphing. Not a cheap equaliser bar. Think fluid simulation.
- **Processing / Thinking**: The orb shifts into a different motion — a slow, inward spiral or a different rhythm. The colour shifts slightly cooler or dimmer. "I'm working on it."
- **Speaking**: A distinct output animation. The orb pulses outward with the speech rhythm. Audio waveform visualisation beneath or around it.
- **Error / Disconnected**: Desaturated, smaller. A quiet signal that something's wrong.

The orb should be **the whole screen** in voice mode — full bleed, immersive. The agent name appears above it, subtle. A stop/end button appears below it, minimal.

When transitioning from voice mode back to text, the orb collapses back into the message thread and the transcript of the conversation appears as messages.

### 4. Model Switcher

Users can switch between AI models mid-conversation (e.g. Claude Opus, Claude Haiku, future models).

- Accessible via a **subtle chip/badge** near the agent name in the conversation header — tapping it opens a bottom sheet or popover.
- Current model displayed at all times but quietly — never the loudest thing on screen.
- Model list: icon or letter avatar, name, short descriptor (e.g. "Most capable" / "Fastest / Cheapest").
- Switching model mid-conversation is instant — a brief confirmation toast or a subtle header animation acknowledges the change.
- Consider a prefix convention display: users can type `opus:` or `haiku:` as a shorthand — hint this in the UI subtly (a ghost text or a tooltip the first time).

### 5. Agent Switcher (Mid-Conversation)

Users may want to switch agents without losing their message history context.

- A **swipe gesture** on the conversation header or a persistent but subtle icon opens an agent selector.
- The switch should feel like changing channels — a horizontal swipe animation that carries the message thread into a new "context".
- A brief system message appears: `Switched to Marketing agent · Opus`
- The UI's ambient colour shifts to the new agent's identity colour smoothly.

### 6. Session / History View

A simple log of past conversations across agents.

- Chronological list, grouped by date.
- Each item: agent colour dot, first message preview, timestamp, model used.
- Swipe to delete.
- Tap to resume conversation in full context.
- Search bar at top.

---

## Key Interaction Principles

### Voice is Primary
The voice button should be **immediately reachable** with one thumb, without re-gripping the phone. Bottom-right or centre-bottom — thumb zone. It should be larger and more visually distinct than the text send button.

### Speed is a Feature
Transitions: 200–350ms. Snappy. The app should feel faster than it needs to because it respects the user's time. Use spring physics on element entrances, not linear easing.

### Minimal Persistent UI
At any given moment, the minimum number of UI elements should be visible. The conversation is the product. Controls appear on demand (on tap, on focus, on scroll) and retreat when not needed.

### Haptics
Design with haptic feedback in mind:
- Soft tap when starting voice recording
- Medium tap when a response completes
- Error haptic for disconnected states
Note these in the design specs so engineering implements them correctly.

### Empty States
Every screen needs a beautiful, considered empty state:
- New conversation: a centred agent avatar/orb, a soft greeting, 3 suggested prompts in muted chips
- No history: minimal illustration or just type, not a sad empty box
- No connection: clear, calm error state with retry option — not alarming, just informative

---

## Navigation & Information Architecture

Keep it flat. Maximum 2 levels deep.

```
Home (Agent Select)
  └── Conversation (Text + Voice)
        ├── Model Switcher (bottom sheet)
        └── Agent Switcher (swipe / bottom sheet)

History (slide in from left or bottom tab)
  └── Past Conversation → resumes Conversation view

Settings (accessible from Home, minimal)
  ├── Account / API connection
  ├── Voice settings (speed, voice selection)
  └── Appearance (Dark only, accent options)
```

Consider **no bottom tab bar** — instead, use gesture navigation:
- Swipe right from conversation to go back to Home
- Swipe left on Home to open History
- Long press agent to see info/edit

---

## Components to Design

Deliver full component coverage for:

- [ ] Agent card (default, selected, active states)
- [ ] Message bubble — user (right-aligned)
- [ ] Message block — agent (left, streaming, complete)
- [ ] Code block (within messages)
- [ ] Voice orb (all 5 states: idle, listening, thinking, speaking, error)
- [ ] Input bar (collapsed, expanded, recording state)
- [ ] Model selector chip + popover/sheet
- [ ] Agent switcher sheet
- [ ] System message (e.g. "Switched to Marketing · Haiku")
- [ ] Toast / notification
- [ ] Navigation header (conversation view)
- [ ] History list item
- [ ] Empty states (conversation, history)
- [ ] Settings rows
- [ ] Loading / skeleton states

---

## Deliverables Expected

1. **High-fidelity Figma frames** for all screens listed above
2. **Prototype** with core flows: open app → select agent → send text message → switch to voice mode → end voice session → switch agent
3. **Component library** in Figma with variants and interactive states
4. **Motion spec** — annotate key transitions and animations (orb states, screen transitions, message streaming, agent/model switching)
5. **Design tokens** — colours, type scale, spacing, shadow, radius — exported in a format usable by the frontend (CSS variables or JSON)
6. **Dark mode only** for V1

---

## Technical Context (for reference)

The app will connect to the Winston backend via:
- **REST API** (`/api/agents`, `/api/agents/{id}/run`, `/api/agents/{id}/sessions`)
- **Voice endpoints** (`/api/voice/transcribe` for STT, `/api/voice/speak` for TTS via ElevenLabs)
- **Basic Auth** (username + password stored in app keychain)
- The backend runs on the user's own Mac, exposed via Cloudflare Tunnel

This means:
- There is **latency** on agent responses — design for it. Streaming text and animated orb states exist to fill this gracefully.
- **Connection state** matters — the app may be offline or the server may be unreachable. Design clear but calm disconnected states.
- **Model names** are real Claude models: `claude-opus-4-5`, `claude-haiku-4-5`, etc. Display cleanly, not raw strings.

---

## What Success Looks Like

A designer, developer, or investor picks up the phone and within 3 seconds understands:
1. This is an AI agent app
2. Voice is the primary interaction
3. It's personal, premium, and powerful

Someone who uses Claude daily and ChatGPT's voice mode looks at this and thinks: *"This is what those apps should have built."*

The app should feel like it belongs in a pocket, not on a server rack — even though it's powered by one.
