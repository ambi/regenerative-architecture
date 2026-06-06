import "@mantine/core/styles.css";
import "./styles.css";

import { MantineProvider } from "@mantine/core";
import { RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { readPageData } from "./page-data";
import { createAppRouter } from "./router";
import { theme } from "./theme";

const pageData = readPageData();
const router = createAppRouter(pageData);
const root = document.getElementById("root");

if (!root) {
  throw new Error("RA Identity root element is missing");
}

createRoot(root).render(
  <StrictMode>
    <MantineProvider theme={theme} defaultColorScheme="light">
      <RouterProvider router={router} />
    </MantineProvider>
  </StrictMode>,
);
