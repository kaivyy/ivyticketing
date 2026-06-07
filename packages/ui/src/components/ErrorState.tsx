import { Button } from "./Button";

export interface ErrorStateProps {
  title?: string;
  message?: string;
  onRetry?: () => void;
}

export function ErrorState({ title = "Something went wrong", message, onRetry }: ErrorStateProps) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-danger/30 bg-danger/5 p-10 text-center">
      <p className="font-medium text-danger">{title}</p>
      {message && <p className="mt-1 text-sm text-gray-600">{message}</p>}
      {onRetry && <Button className="mt-4" variant="danger" onClick={onRetry}>Retry</Button>}
    </div>
  );
}
