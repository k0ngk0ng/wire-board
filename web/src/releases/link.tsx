import "./link.css";

export function ReleaseVersionLink({
  version,
  className = "",
}: {
  version?: string;
  className?: string;
}) {
  const value = (version || "dev").trim();
  const label = value === "dev" || value.startsWith("v") ? value : `v${value}`;
  return (
    <a
      className={`release-version-link ${className}`}
      href={/^v\d+\.\d+\.\d+$/.test(label) ? `/releases/${label}` : "/releases"}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={`查看 ${label} 更新日志（新标签页）`}
    >
      {label}
    </a>
  );
}
