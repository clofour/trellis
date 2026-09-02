"use client";

import { createContext, useContext, useEffect, useState } from "react";

type DashboardAPIAccess = "namespace" | "cluster";

const namespacePattern = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$/;

interface Config {
  allowWrites: boolean;
  apiAccess: DashboardAPIAccess;
  clusterName: string;
  namespace: string;
  namespaces: string[];
  allowAnyNamespace: boolean;
  setNamespace: (namespace: string) => void;
}

const ConfigContext = createContext<Config>({
  allowWrites: false,
  apiAccess: "namespace",
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
  const [namespace, setSelectedNamespace] = useState(defaultNamespace);

  useEffect(() => {
    let cancelled = false;

    const loadScope = async () => {
      try {
        const response = await fetch("/api/v1/access", { cache: "no-store" });
        if (!response.ok) return;
        const data = (await response.json()) as {
          api_access?: DashboardAPIAccess;
        };
        if (
          cancelled ||
          (data.api_access !== "cluster" && data.api_access !== "namespace")
        ) {
          return;
        }
        setAPIAccess(data.api_access);
      } catch {
        // Keep the safe namespace-only default when scope detection is unavailable.
      }
    };

    void loadScope();
    return () => {
      cancelled = true;
    };
  }, []);

  const setNamespace = (next: string) => {
    const candidate = next.trim();
    if (!namespacePattern.test(candidate)) return;
    if (!allowAnyNamespace && !namespaces.includes(candidate)) return;
    setSelectedNamespace(candidate);
  };

  return (
    <ConfigContext.Provider
      value={{
        allowWrites,
        apiAccess,
        clusterName,
        namespace,
        namespaces,
        allowAnyNamespace,
        setNamespace,
      }}
    >
      {children}
    </ConfigContext.Provider>
  );
}
