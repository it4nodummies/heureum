// Default project avatar: no network request, no broken <img>. Renders a
// rounded square filled with a color deterministically derived from the
// project key/name, containing its initials. Real uploaded project avatars
// aren't a thing yet (avatarUrls always 404s), so this is the only avatar.

const PALETTE = [
  "#0052cc", // blue (accent)
  "#7c3aed", // violet
  "#00875a", // green
  "#de350b", // red
  "#c2410c", // orange
  "#0e7490", // teal
];

function hashString(value: string): number {
  let h = 0;
  for (let i = 0; i < value.length; i++) {
    h = (h << 5) - h + value.charCodeAt(i);
    h |= 0;
  }
  return Math.abs(h);
}

interface Props {
  nameOrKey: string;
  size?: number;
}

export function ProjectAvatar({ nameOrKey, size = 44 }: Props) {
  const label = (nameOrKey || "?").trim();
  const initials = (label.slice(0, 2).toUpperCase() || "?").replace(/[^A-Z0-9]/g, "") || "?";
  const color = PALETTE[hashString(label) % PALETTE.length];
  const fontSize = Math.max(10, Math.round(size * 0.4));

  return (
    <div
      aria-hidden="true"
      className="rounded-lg flex items-center justify-center text-white font-bold shrink-0 select-none"
      style={{ width: size, height: size, background: color, fontSize }}
    >
      {initials}
    </div>
  );
}
