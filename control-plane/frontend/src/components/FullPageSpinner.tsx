import { Loader2 } from "lucide-react";

/**
 * Shown while the app's initial auth/setup queries are in flight. Previously
 * these gates rendered `null`, which is indistinguishable from a crash — a
 * blank white screen. A visible spinner makes "still loading" obviously
 * different from "broken", and pairs with the bounded request timeouts so a
 * genuinely stuck connection resolves into an error page rather than spinning
 * forever.
 */
export default function FullPageSpinner() {
  return (
    <div
      className="min-h-screen flex items-center justify-center bg-gray-50"
      role="status"
      aria-label="Loading"
    >
      <Loader2 className="h-8 w-8 animate-spin text-gray-400" />
    </div>
  );
}
