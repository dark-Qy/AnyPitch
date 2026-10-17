/** Shared full-size pitch coordinates keep the preview and drag surface aligned. */
export function PitchMarkings() {
  return <svg className="pitch-markings" viewBox="0 0 68 105" aria-hidden="true">
    <g fill="none" stroke="currentColor" strokeWidth="0.35">
      <rect x="0.5" y="0.5" width="67" height="104" />
      <path d="M.5 52.5H67.5 M13.84 .5V16.5H54.16V.5 M24.84 .5V5.5H43.16V.5 M13.84 104.5V88.5H54.16V104.5 M24.84 104.5V99.5H43.16V104.5" />
      <circle cx="34" cy="52.5" r="9.15" />
      <path d="M26.69 16.5A9.15 9.15 0 0 0 41.31 16.5 M26.69 88.5A9.15 9.15 0 0 1 41.31 88.5" />
      <path d="M.5 1.5A1 1 0 0 0 1.5 .5 M66.5 .5A1 1 0 0 0 67.5 1.5 M.5 103.5A1 1 0 0 1 1.5 104.5 M66.5 104.5A1 1 0 0 1 67.5 103.5" />
    </g>
    <g fill="currentColor"><circle cx="34" cy="52.5" r=".45"/><circle cx="34" cy="11" r=".45"/><circle cx="34" cy="94" r=".45"/></g>
  </svg>;
}
