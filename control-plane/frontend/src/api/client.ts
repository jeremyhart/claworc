import axios from "axios";

const client = axios.create({
  baseURL: "/api/v1",
  // Without a timeout, a request whose response is silently dropped by the
  // network (common on flaky mobile / VPN connections) hangs forever. The app
  // gates its first render on the auth/setup queries, so a hung request leaves
  // React stuck rendering nothing — a blank white screen with no error. A
  // bounded timeout turns that into a normal failure the UI can react to.
  timeout: 20000,
});

client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (
      error.response?.status === 401 &&
      !error.config?.url?.includes("/auth/")
    ) {
      window.location.href = "/login";
    }
    return Promise.reject(error);
  },
);

export default client;
