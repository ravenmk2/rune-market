import { harnessLabel } from "../utils/format";

export function HarnessChip({ id }: { id: string }) {
  return <span className="harness-chip">{harnessLabel(id)}</span>;
}
