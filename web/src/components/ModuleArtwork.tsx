import "./ModuleArtwork.css";

/** 手绘魔法书与羽毛笔装饰。 */
export function ModuleArtwork() {
  return <div className="module-artwork" aria-hidden="true">
    <svg viewBox="0 0 340 240" fill="none" focusable="false">
      <g strokeLinecap="round" strokeLinejoin="round">
        <g className="book-outline" strokeWidth="3.2">
          <path d="M35 103c29-12 61-7 106 13 32-22 70-31 113-19l-5 104c-42-12-77-4-109 14-35-19-70-25-112-12l7-100Z" fill="var(--paper)" />
          <path d="M35 103c2-5 4-8 8-9 35-8 65 1 98 22 34-25 70-36 106-27l7 8M141 116c-4 27-1 65-1 99" />
          <path d="M28 203c1 5 3 8 8 9 36-9 70-3 104 13 34-17 70-24 109-13l4-10M140 215v10" />
        </g>
        <g className="book-text" strokeWidth="2">
          <path d="M58 123c9-7 20-4 23 4s-4 17-13 14-14-13-6-19Z" />
          <path d="m69 119-2 21m-10-13 23 5M91 130l19 7m-18 2 11 4" />
          <path d="m55 157 10-7 11 11-10 9-11-13Zm21 4 13 3m-3-4 3 4-5 3" />
          <path d="M96 157c9 0 14 5 17 10l-8 7-13-8 4-9Z" />
          <path d="M51 183c7-6 8 7 14 1s6 6 12 2M86 188l17 4" />
        </g>
        <g className="book-writing" strokeWidth="2.8">
          <path className="written-line line-one" pathLength="1" d="M164 140c8-4 15-10 20-5s10-10 16-6 12-5 18-3" />
          <path className="written-line line-two" pathLength="1" d="M163 159c8-5 14-9 20-5s12-9 18-6 10-4 16-3" />
          <path className="written-line line-three" pathLength="1" d="M162 178c10-7 16-6 22-9s13-2 19-4" />
        </g>
        <g className="writing-pen" strokeWidth="3">
          <path d="M174 124c-4-19 7-43 25-58 18-16 43-29 72-26 8 1 14 6 13 11-2 8-18 10-31 9 6 4 11 10 8 15-4 6-18 4-27 1 4 6 7 12 2 16-5 5-15 0-20-2 1 7 1 14-5 18-7 5-13 1-17 1-6 4-11 9-20 15Z" fill="var(--paper)" />
          <path d="M164 140c23-38 49-65 86-86" />
          <path d="m197 94 4-17m13 0 15-3" strokeWidth="1.7" />
        </g>
        <g className="book-spark spark-one" strokeWidth="1.6"><path d="M106 40c1 13 4 16 15 18-11 2-14 5-15 18-2-13-5-16-16-18 11-2 14-5 16-18Z" /></g>
        <g className="book-spark spark-two" strokeWidth="1.4"><path d="M291 110c1 9 3 11 10 13-7 1-9 4-10 13-1-9-4-12-11-13 7-2 10-4 11-13Z" /></g>
        <g className="book-spark spark-three" strokeWidth="1.2"><path d="M69 67c1 6 2 8 7 9-5 1-6 3-7 9-1-6-3-8-8-9 5-1 7-3 8-9Z" /></g>
      </g>
    </svg>
  </div>;
}
