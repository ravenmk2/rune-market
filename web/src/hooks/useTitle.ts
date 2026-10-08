import { useEffect } from "react";

const DEFAULT_TITLE = "RuneMarket";

/** 设置页面标题,卸载/离开时恢复默认 */
export function useTitle(title?: string) {
  useEffect(() => {
    if (title) document.title = title;
    return () => {
      document.title = DEFAULT_TITLE;
    };
  }, [title]);
}
