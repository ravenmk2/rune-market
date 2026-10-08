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

function collectDirs(nodes: TreeNode[], out: Set<string>): Set<string> {
  for (const n of nodes) {
    if (n.isDir) {
      out.add(n.path);
      collectDirs(n.children, out);
    }
  }
  return out;
}

interface FileTreeProps {
  paths: string[];
  active: string | null;
  onSelect: (path: string) => void;
}

function FolderIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true" fill="currentColor">
      <path d="M1.75 2.5a1 1 0 0 0-1 1v9a1 1 0 0 0 1 1h12.5a1 1 0 0 0 1-1v-7a1 1 0 0 0-1-1H7.6L6.3 3.1a1 1 0 0 0-.8-.6H1.75Z" />
    </svg>
  );
}

function FileIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true" fill="currentColor">
      <path
        fillRule="evenodd"
        d="M3.5 1.5a1 1 0 0 0-1 1v11a1 1 0 0 0 1 1h9a1 1 0 0 0 1-1V5.2L9.8 1.5H3.5Zm5.8 1 3.2 3.2H9.3V2.5Z"
      />
    </svg>
  );
}

/** 扁平路径列表建树,目录默认全部折叠、点击展开(§9:附属文件原样出现在树中) */
export function FileTree({ paths, active, onSelect }: FileTreeProps) {
  const tree = useMemo(() => buildTree(paths), [paths]);
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(() =>
    collectDirs(tree, new Set()),
  );

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
                  <FolderIcon />
                  {n.name}/
                </span>
                {!collapsed.has(n.path) && renderNodes(n.children)}
              </>
            ) : (
              <span
                className={`node${active === n.path ? " active" : ""}`}
                onClick={() => onSelect(n.path)}
              >
                <span className="tw"></span>
                <FileIcon />
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
