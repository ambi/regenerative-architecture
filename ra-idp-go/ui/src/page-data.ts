import type { PageData } from "./types";

export function readPageData(): PageData {
  const element = document.getElementById("ra-page-data");
  if (!element?.textContent) {
    throw new Error("RA Identity page data is missing");
  }
  return JSON.parse(element.textContent) as PageData;
}
