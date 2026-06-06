export type LoginPage = {
  kind: "login";
  requestId: string;
  error?: string;
};

export type ConsentPage = {
  kind: "consent";
  requestId: string;
  clientName: string;
  scope: string;
};

export type DevicePage = {
  kind: "device";
  userCode: string;
};

export type StatusPage = {
  kind: "status";
  status: "approved" | "denied" | "signed-out" | "authentication-required";
};

export type PageData = LoginPage | ConsentPage | DevicePage | StatusPage;
