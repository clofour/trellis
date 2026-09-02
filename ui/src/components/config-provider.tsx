"use client";

import { createContext, useContext, useEffect, useState } from "react";

type DashboardAPIAccess = "namespace" | "cluster";
type DashboardAccessLevel = "read" | "write";

const namespacePattern = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$/;

interface Config {
  allowWrites: boolean;
  apiAccess: DashboardAPIAccess;
  accessLevel: DashboardAccessLevel;
  clusterName: string;
  namespace: string;
  namespaces: string[];
  allowAnyNamespace: boolean;
  setNamespace: (namespace: string) => void;
}

const ConfigContext = createContext<Config>({
  allowWrites: false,
  apiAccess: "namespace",
  accessLevel: "read",
  clusterName: "Trellis cluster",
  namespace: "",
  namespaces: [""],
  allowAnyNamespace: false,
  setNamespace: () => undefined,
});

export function useConfig(): Config {
  return useContext(ConfigContext);
}

export function ConfigProvider({
  allowWrites,
  clusterName,
  defaultNamespace,
  namespaces,
  allowAnyNamespace,
  children,
}: {
  allowWrites: boolean;
  clusterName: string;
  defaultNamespace: string;
  namespaces: string[];
  allowAnyNamespace: boolean;
  children: React.ReactNode;
}) {
  const [apiAccess, setAPIAccess] = useState<DashboardAPIAccess>("namespace");
  const [accessLevel, setAccessLevel] = useState<DashboardAccessLevel>("read");
  const [namespace, setSelectedNamespace] = useState(defaultNamespace);

  useEffect(() => {
    let cancelled = false;

    const loadCredential = async () => {
      try {
        const response = await fetch("/api/v1/access", { cache: "no-store" });
        if (!response.ok) return;
        const data = (await response.json()) as {
          scope?: DashboardAPIAccess;
          access?: DashboardAccessLevel;
          namespace?: string;
        };
        if (
          cancelled ||
          (data.scope !== "cluster" && data.scope !== "namespace") ||
          (data.access !== "read" && data.access !== "write")
        ) {
          return;
        }
        setAPIAccess(data.scope);
        setAccessLevel(data.access);
        if (
          data.scope === "namespace" &&
          data.namespace &&
          namespacePattern.test(data.namespace)
        ) {
          setSelectedNamespace(data.namespace);
        }
      } catch {
        // Keep safe namespace/read defaults when credential introspection is unavailable.
      }
    };

    void loadCredential();
    return () => {
      cancelled = true;
    };
  }, []);

  const setNamespace = (next: string) => {
    if (apiAccess !== "cluster") return;
    const candidate = next.trim();
    if (!namespacePattern.test(candidate)) return;
    if (!allowAnyNamespace && !namespaces.includes(candidate)) return;
    setSelectedNamespace(candidate);
  };

  return (
    <ConfigContext.Provider
      value={{
        allowWrites: allowWrites && accessLevel === "write",
        apiAccess,
        accessLevel,
        clusterName,
        namespace,
        namespaces,
        allowAnyNamespace: allowAnyNamespace && apiAccess === "cluster",
        setNamespace,
      }}
    >
      {children}
    </ConfigContext.Provider>
  );
}
