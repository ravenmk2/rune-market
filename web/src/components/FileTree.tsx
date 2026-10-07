import { useMemo, useState } from "react";

interface TreeNode {
  name: string;
  path: string;
  isDir: boolean;
  children: TreeNode[];
}

function buildTree(paths: string[]): TreeNode[] {
  const root: TreeNode = { name: "", path: "", isDir: true, children: [] };
  for (const p of paths) {
    const segs = p.split("/").filter(Boolean);
    let node = root;
    let cur = "";
    segs.forEach((seg, i) => {
      cur = cur ? `${cur}/${seg}` : seg;
      const isDir = i < segs.length - 1;
      let child = node.children.find((c) => c.name === seg && c.isDir === isDir);
      if (!child) {
        child = { name: seg, path: cur, isDir, children: [] };
        node.children.push(child);
      }
      node = child;
    });
  }
  const sortRec = (nodes: TreeNode[]) => {
    nodes.sort((a, b) =>
      a.isDir === b.isDir ? a.name.localeCompare(b.name) : a.isDir ? -1 : 1,
    );
    nodes.forEach((n) => sortRec(n.children));
  };
  sortRec(root.children);
  return root.children;
}

interface FileTreeProps {
  paths: string[];
  active: string | null;
  onSelect: (path: string) => void;
}

/** 扁平路径列表建树,目录可折叠(§9:附属文件原样出现在树中) */
export function FileTree({ paths, active, onSelect }: FileTreeProps) {
  const tree = useMemo(() => buildTree(paths), [paths]);
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set());

  function toggle(path: string) {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  }

  function renderNodes(nodes: TreeNode[]) {
    return (
      <ul>
        {nodes.map((n) => (
          <li key={n.path}>
            {n.isDir ? (
              <>
                <span className="node" onClick={() => toggle(n.path)}>
                  <span className="tw">{collapsed.has(n.path) ? "▸" : "▾"}</span>
                  {n.name}/
                </span>
                {!collapsed.has(n.path) && renderNodes(n.children)}
              </>
            ) : (
              <span
                className={`node${active === n.path ? " active" : ""}`}
                onClick={() => onSelect(n.path)}
              >
                <span className="tw">·</span>
                {n.name}
              </span>
            )}
          </li>
        ))}
      </ul>
    );
  }

  return <aside className="filetree">{renderNodes(tree)}</aside>;
}
