declare const process: {
  env: Record<string, string | undefined>;
  stdout: { write: (chunk: string | Uint8Array) => void };
  exit: (code?: number) => never;
};
