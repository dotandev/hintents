// Minimal Node shims for environments where @types/node is not installed.
// Prefer installing `@types/node` and keeping tsconfig `types: ["node"]`.

declare const process: {
  env: Record<string, string | undefined>;
};

declare class Buffer extends Uint8Array {
  static from(data: any, encoding?: string): Buffer;
  toString(encoding?: string): string;
}
