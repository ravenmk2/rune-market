interface PaginationProps {
  page: number;
  pageSize: number;
  total: number;
  onChange: (page: number) => void;
}

function pageNumbers(page: number, pages: number): (number | "…")[] {
  if (pages <= 7) return Array.from({ length: pages }, (_, i) => i + 1);
  const set = new Set<number>([1, 2, page - 1, page, page + 1, pages - 1, pages]);
  const nums = [...set].filter((n) => n >= 1 && n <= pages).sort((a, b) => a - b);
  const out: (number | "…")[] = [];
  nums.forEach((n, i) => {
    if (i > 0 && n - nums[i - 1] > 1) out.push("…");
    out.push(n);
  });
  return out;
}

export function Pagination({ page, pageSize, total, onChange }: PaginationProps) {
  const pages = Math.ceil(total / pageSize);
  if (pages <= 1) return null;
  return (
    <div className="pagination">
      <button
        className={`page-btn${page <= 1 ? " disabled" : ""}`}
        onClick={() => onChange(page - 1)}
      >
        ‹
      </button>
      {pageNumbers(page, pages).map((n, i) =>
        n === "…" ? (
          <span key={`e${i}`} className="page-btn disabled">
            …
          </span>
        ) : (
          <button
            key={n}
            className={`page-btn${n === page ? " active" : ""}`}
            onClick={() => onChange(n)}
          >
            {n}
          </button>
        ),
      )}
      <button
        className={`page-btn${page >= pages ? " disabled" : ""}`}
        onClick={() => onChange(page + 1)}
      >
        ›
      </button>
    </div>
  );
}
