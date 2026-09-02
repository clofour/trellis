"use client";

import { createContext, useContext, useEffect, useState } from "react";

type DashboardAPIAccess = "namespace" | "cluster";

interface Config {
  allowWrites: boolean;
  apiAccess: DashboardAPIAccess;
  clusterName: string;
  namespace: string;
  namespaces: string[];
  setNamespace: (namespace: string) => void;
}

const ConfigContext = createContext<Config>({
  allowWrites: false,
  apiAccess: "namespace",
  clusterName: "Trellis cluster",
  namespace: "",
  namespaces: [""],
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
  children,
}: {
  allowWrites: boolean;
  clusterName: string;
  defaultNamespace: string;
  namespaces: string[];
  children: React.ReactNode;
}) {
  const [apiAccess, setAPIAccess] = useState<DashboardAPIAccess>("namespace");
  const [namespace, setSelectedNamespace] = useState(defaultNamespace);
  const [namespaceOptions, setNamespaceOptions] = useState(namespaces);

  useEffect(() => {
    let cancelled = false;

    const loadScope = async () => {
      try {
        const accessResponse = await fetch("/api/v1/access", { cache: "no-store" });
        if (!accessResponse.ok) return;
        const access = (await accessResponse.json()) as {
          api_access?: DashboardAPIAccess;
        };
        if (cancelled || (access.api_access !== "cluster" && access.api_access !== "namespace")) {
          return;
        }
        setAPIAccess(access.api_access);
        if (access.api_access !== "cluster") return;

        const namespacesResponse = await fetch("/api/v1/namespaces", {
          cache: "no-store",
        });
        if (!namespacesResponse.ok) return;
        const discovered = (await namespacesResponse.json()) as unknown;
        if (!Array.isArray(discovered)) return;
        const options = discovered.filter(
          (item): item is string => typeof item === "string" && item.length > 0,
        );
        if (cancelled || options.length === 0) return;
        setNamespaceOptions(options);
        setSelectedNamespace((current) =>
          options.includes(current) ? current : options[0],
        );
      } catch {
        // Keep the safe namespace-only defaults when scope discovery is unavailable.
      }
    };

    void loadScope();
    return () => {
      cancelled = true;
    };
  }, []);

  const setNamespace = (next: string) => {
    if (!namespaceOptions.includes(next)) return;
    setSelectedNamespace(next);
  };

  return (
    <ConfigContext.Provider
      value={{
        allowWrites,
        apiAccess,
        clusterName,
        namespace,
        namespaces: namespaceOptions,
        setNamespace,
      }}
    >
      {children}
    </ConfigContext.Provider>
  );
}
