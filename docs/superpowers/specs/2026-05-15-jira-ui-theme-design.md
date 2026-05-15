# Jira-Like UI Theme — Design Document

> **Data:** 2026-05-15
> **Riferimento:** Atlassian Design System (ADS) — atlassian.design
> **Base:** Tailwind CSS v4 su React/TypeScript/Vite

---

## Obiettivo

Riprodurre l'estetica di Atlassian Jira Cloud sull'applicazione Open Jira, utilizzando la palette ufficiale ADS dark theme tramite override dei design token di Tailwind CSS v4.

---

## Palette Colori (ADS Dark Theme)

### Sfondo e Superfici

| Token Tailwind | Valore ADS | Uso |
|---------------|-----------|-----|
| `bg-surface` | `#1D2125` (DarkNeutral100) | Sfondo principale (board, backlog, pagine) |
| `bg-sidebar` | `#161A1D` (DarkNeutral0) | Sidebar sinistra e top bar |
| `bg-card` | `#22272B` (DarkNeutral200) | Card issue, pannelli, widget |
| `bg-card-hover` | `#2C333A` (DarkNeutral300) | Hover card |
| `bg-elevated` | `#282E33` (DarkNeutral250) | Dropdown, popover, modali |
| `bg-input` | `#22272B` (DarkNeutral200) | Input field, search bar |

### Bordi

| Token Tailwind | Valore ADS | Uso |
|---------------|-----------|-----|
| `border-default` | `#2C333A` (DarkNeutral300) | Bordi standard |
| `border-focus` | `#579DFF` (Blue400) | Bordi focus input |

### Testo

| Token Tailwind | Valore ADS | Uso |
|---------------|-----------|-----|
| `text-primary` | `#B6C2CF` (DarkNeutral900) | Testo principale, titoli |
| `text-secondary` | `#8C9BAB` (DarkNeutral700) | Metadati, date, chiavi issue |
| `text-inverse` | `#1D2125` (DarkNeutral100) | Testo su sfondo bold (badge, pulsanti) |
| `text-link` | `#579DFF` (Blue400) | Link |
| `text-subtlest` | `#596773` (DarkNeutral500) | Testo disabilitato, placeholder |

### Accent / Semantici

| Token Tailwind | Valore ADS | Uso |
|---------------|-----------|-----|
| `accent-blue` | `#579DFF` (Blue400) | Primary button, link, selezione |
| `accent-green` | `#2ABB7F` (Green400) | Status DONE, success |
| `accent-yellow` | `#E2B203` (Yellow500) | Status IN PROGRESS, warning |
| `accent-red` | `#E2483D` (Red600) | Priority HIGHEST, danger, error |
| `accent-orange` | `#F18D0D` (Orange500) | Priority HIGH |
| `accent-purple` | `#9F8FEF` (Purple400) | Discovery/new features |
| `accent-teal` | `#3E9FA0` (Teal500) | Accent generici |

### Status Workflow

| Stato | Colore | Uso |
|-------|--------|-----|
| TO DO | `#8C9BAB` (DarkNeutral700) | Colonna grigia |
| IN PROGRESS | `#579DFF` (Blue400) | Colonna blu |
| DONE | `#2ABB7F` (Green400) | Colonna verde |
| Custom | `#9F8FEF` | Personalizzati |

### Priority

| Priority | Colore | Icona |
|----------|--------|-------|
| Highest | `#E2483D` (Red600) | Freccia rossa ↑↑↑ |
| High | `#F18D0D` (Orange500) | Freccia arancione ↑↑ |
| Medium | `#E2B203` (Yellow500) | Freccia gialla ↑ |
| Low | `#2ABB7F` (Green400) | Freccia verde ↓ |
| Lowest | `#8C9BAB` (DarkNeutral700) | Freccia grigia ↓↓ |

---

## Tipografia

```css
font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
```

- **Titoli:** 14px/600 (semibold)
- **Body:** 14px/400 (regular)
- **Chiave issue:** 14px/600, colore `text-secondary`
- **Metadati:** 12px/400, colore `text-secondary`

---

## Struttura Layout

```
┌─────────────────────────────────────────────────────────────┐
│ TOP BAR: bg-sidebar, h-48px                                 │
│ [☰] [Jira Logo] Projects ▼ | [🔍 Search...] | [+] Create  │
├──────────┬──────────────────────────────────────────────────┤
│ SIDEBAR  │  CONTENT AREA: bg-surface                       │
│ bg-sidebar│ ┌─ Board / Backlog ──────────────────────────┐ │
│ w-240px  │ │ ┌────────┐ ┌──────────┐ ┌──────┐          │ │
│          │ │ │ TO DO 2│ │IN PROG 1 │ │DONE 0│          │ │
│ Navigaz. │ │ │────────│ │──────────│ │──────│          │ │
│ · Boards │ │ │ Card   │ │ Card     │ │      │          │ │
│ · Backlog│ │ │ Card   │ │          │ │      │          │ │
│ · Reports│ │ └────────┘ └──────────┘ └──────┘          │ │
│ · Issues │ └────────────────────────────────────────────┘ │
└──────────┴──────────────────────────────────────────────────┘
```

## Specifiche per Componente

### Top Bar
- Altezza: 48px
- Background: `bg-sidebar` (#161A1D)
- Logo: testo "Open Jira" in bianco, 20px, bold
- Breadcrumb: `Projects / OJ / Board`
- Search: input `bg-input` con bordo `border-default`, rounded-[3px], placeholder "Search..."
- Create button: `bg-accent-blue`, testo bianco, rounded-[3px], h-32px
- Avatar utente: cerchio 32px con iniziali, `bg-[#2C333A]`

### Sidebar
- Larghezza: 240px
- Background: `bg-sidebar` (#161A1D)
- Sezioni: Projects (con lista progetti), Boards, Filters
- Item attivo: `bg-[#1C2B41]` (blue tint), bordo sinistro `accent-blue` (3px)
- Item normale: `text-secondary`, hover `bg-[#22272B]`
- Icone: 16px, `text-secondary`

### Board
- Colonna header: `bg-surface`, titolo uppercase 12px/600 `text-secondary`
- Badge conteggio: `bg-[#38414A]`, `rounded-full`, `text-xs`, `px-2`, altezza 20px
- Card Issue:
  - `bg-card`, `rounded-[3px]`, `p-3` (12px), `shadow-sm`
  - Hover: `bg-card-hover`, `shadow-md`
  - Key: `text-secondary`, 12px, esempio "OJ-1"
  - Title: `text-primary`, 14px/500, max 2 righe
  - Metadati in basso: tipo (icona), priority (icona colorata), assignee (avatar), story points (badge)
  - Drag handle: icona grip a sinistra (visibility:hidden, show on hover)

### Backlog
- Split view: sinistra (backlog list), destra (sprint panel)
- Sprint panel: `bg-card`, `rounded-[3px]`, header con nome sprint + date + goal
- Issue list: righe `bg-surface`, hover `bg-card-hover`, `border-b border-default`
- Drag per riordinare
- Pulsante "Create sprint": `bg-accent-blue`
- Pulsante "Create issue": icona [+] inline

### Issue Detail
- Header: chiave issue (OJ-1), titolo (textarea 24px), breadcrumb
- Campi editabili: assignee, priority, labels, sprint, story points — layout a griglia 2-colonne
- Ogni campo: label 12px `text-secondary`, valore/input 14px `text-primary`, `border-default` bottom
- Descrizione: area TipTap editor con toolbar floating
- Attività: tabs (Commenti | History | Git)
  - Commenti: avatar + nome + timestamp + body
  - History: timeline con pallino colorato, "X changed Y from Z to W"

### Card Componente Riutilizzabile
```
┌──────────────────────────────────────┐
│ [type icon] OJ-1                     │ ← 12px, text-secondary
│ Setup homelab deployment             │ ← 14px/500, text-primary
│                                      │
│ 🏷 task  ↑↑ high  👤 AM   ⭐ 3       │ ← metadati
└──────────────────────────────────────┘
```

### Priority Icons (Invece di emoji)
- Highest: freccia su rossa `▲▲▲` (o icona Lucide `ArrowBigUp`)
- High: freccia su arancione `▲▲`
- Medium: freccia su gialla `▲`
- Low: freccia giu verde `▼`
- Lowest: freccia giu grigia `▼▼`

---

## Piano di Implementazione

### Step 1: Design Tokens (Tailwind @theme)
Sostituire `@import "tailwindcss"` con `@theme` block in `index.css` con tutte le variabili ADS.

### Step 2-8: Refactor pagina per pagina
Ogni pagina viene riscritta usando i nuovi token ADS e seguendo il layout Jira:
2. Board
3. Backlog  
4. Issue Detail
5. Dashboard + Widget
6. Search
7. Reports + BurndownChart
8. Timeline + Calendar

### Step 9: Componenti condivisi
- IssueCard, PriorityIcon, Avatar, Badge, TopBar, Sidebar

---

## Self-Review

- [x] Nessun placeholder (TBD/TODO)
- [x] Palette completa con valori esadecimali esatti
- [x] Layout documentato con diagramma ASCII
- [x] Specifiche per ogni pagina e componente
- [x] Piano di implementazione in 9 step
- [x] Riferimenti ADS verificati
