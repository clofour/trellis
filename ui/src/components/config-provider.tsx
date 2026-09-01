"use client";

import { createContext, useContext, useState } from "react";

interface Config {
  allowWrites: boolean;
  clusterName: string;
  namespace: string;
  namespaces: string[];
  setNamespace: (namespace: string) => void;
}

const ConfigContext = createContext<Config>({
  allowWrites: false,
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
  const [namespace, setSelectedNamespace] = useState(defaultNamespace);

  const setNamespace = (next: string) => {
    if (!namespaces.includes(next)) return;
    setSelectedNamespace(next);
  };

  return (
    <ConfigContext.Provider
      value={{ allowWrites, clusterName, namespace, namespaces, setNamespace }}
    >
      {children}
    </ConfigContext.Provider>
  );
}
