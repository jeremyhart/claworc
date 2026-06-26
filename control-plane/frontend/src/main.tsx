import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "react-hot-toast";
import { AuthProvider } from "./contexts/AuthContext";
import { TeamProvider } from "./contexts/TeamContext";
import ErrorBoundary from "./components/ErrorBoundary";
import App from "./App";
import "./index.css";

// Signal to the boot watchdog in index.html that the module bundle loaded and
// executed. If the watchdog still has to show its fallback, this flag tells us
// whether the failure was before JS ran (bundle/parse/network) or after (the
// app mounted but rendered nothing, e.g. a hung request).
(window as unknown as { __CLAWORC_BOOTED__?: boolean }).__CLAWORC_BOOTED__ = true;

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <AuthProvider>
            <TeamProvider>
              <App />
            </TeamProvider>
            <Toaster
              position="bottom-right"
              toastOptions={{
                custom: {
                  style: { padding: 0, background: "transparent", boxShadow: "none" },
                },
              }}
            />
          </AuthProvider>
        </BrowserRouter>
      </QueryClientProvider>
    </ErrorBoundary>
  </StrictMode>,
);
