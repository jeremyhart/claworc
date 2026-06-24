import { Component, type ErrorInfo, type ReactNode } from "react";

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

/**
 * Top-level error boundary. Without this, any uncaught render error leaves the
 * user staring at a blank white <div id="root"> with no information — a
 * "white screen" that is impossible to diagnose, especially on mobile where the
 * dev console isn't readily available. Here we surface the actual error message
 * and offer a reload so a transient failure (e.g. a stale cached bundle after a
 * deploy) can be recovered without clearing site data by hand.
 */
export default class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // Logged to the browser console so the message is retrievable via remote
    // debugging (Safari Web Inspector / chrome://inspect) on a phone.
    console.error("Unhandled UI error:", error, info.componentStack);
  }

  handleReload = () => {
    window.location.reload();
  };

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;

    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center px-4">
        <div className="max-w-md w-full bg-white border border-gray-200 rounded-lg shadow-sm p-6 text-center">
          <h1 className="text-lg font-semibold text-gray-900">
            Something went wrong
          </h1>
          <p className="mt-2 text-sm text-gray-600">
            The app ran into an unexpected error and couldn&apos;t finish
            loading. Reloading often clears this up.
          </p>
          <pre className="mt-4 text-left text-xs text-red-700 bg-red-50 border border-red-100 rounded p-3 overflow-auto max-h-40 whitespace-pre-wrap break-words">
            {error.message}
          </pre>
          <button
            type="button"
            onClick={this.handleReload}
            className="mt-4 inline-flex items-center justify-center rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800"
          >
            Reload
          </button>
        </div>
      </div>
    );
  }
}
