export interface LoadingStateProps {
  label?: string;
}

export function LoadingState({ label = "Loading…" }: LoadingStateProps) {
  return (
    <div role="status" aria-live="polite" className="flex items-center justify-center gap-2 p-10 text-sm text-gray-600">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-primary" />
      {label}
    </div>
  );
}
