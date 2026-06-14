import {
  isRouteErrorResponse,
  Links,
  Meta,
  Outlet,
  Scripts,
  ScrollRestoration,
  type LinksFunction,
} from "react-router";

import type { Route } from "./+types/root";
import stylesheet from "./index.css?url";

export const clientLoader = async () => {};

import backgroundImage from "@/assets/background.webp";
import cloudsImage from "@/assets/clouds.webp";
import reflectionFirstFrameAlpha from "@/assets/reflection-first-frame-alpha.webp";

export const links: LinksFunction = () => [
  { rel: "preconnect", href: "https://fonts.googleapis.com" },
  {
    rel: "preconnect",
    href: "https://fonts.gstatic.com",
    crossOrigin: "anonymous",
  },
  {
    rel: "stylesheet",
    href: "https://fonts.googleapis.com/css2?family=Inter:ital,opsz,wght@0,14..32,100..900;1,14..32,100..900&display=swap",
  },
  { rel: "stylesheet", href: stylesheet },
  { rel: "preload", as: "image", href: backgroundImage },
  { rel: "preload", as: "image", href: cloudsImage },
  { rel: "preload", as: "image", href: reflectionFirstFrameAlpha },
];

const CLOUD_DRIFT_DURATION_SECONDS = 120;

export function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <Meta />
        <Links />
      </head>
      <body
        ref={() => {
          const body = document.body;
          const duration = CLOUD_DRIFT_DURATION_SECONDS;
          const phase = (Date.now() % (duration * 1000)) / 1000;
          body.style.setProperty("--cloud-drift-phase", phase + "s");
        }}
      >
        {children}
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  );
}

export default function App() {
  return <Outlet />;
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  let message = "Oops!";
  let details = "An unexpected error occurred.";
  let stack: string | undefined;

  if (isRouteErrorResponse(error)) {
    message = error.status === 404 ? "404" : "Error";
    details =
      error.status === 404
        ? "The requested page could not be found."
        : error.statusText || details;
  } else if (import.meta.env.DEV && error && error instanceof Error) {
    details = error.message;
    stack = error.stack;
  }

  return (
    <main className="pt-16 p-4 container mx-auto">
      <h1>{message}</h1>
      <p>{details}</p>
      {stack && (
        <pre className="w-full p-4 overflow-x-auto">
          <code>{stack}</code>
        </pre>
      )}
    </main>
  );
}
