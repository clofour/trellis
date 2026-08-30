"use client";

import { createContext, useContext } from "react";

interface Config {
  allowWrites: boolean;
  namespace: string;
}

const ConfigContext = createContext<Config>({ allowWrites: false, namespace: "" });

export function useConfig(): Config {
  return useContext(ConfigContext);
}

export function ConfigProvider({
  allowWrites,
  namespace,
  children,
}: {
  allowWrites: boolean;
  namespace: string;
  children: React.ReactNode;
}) {
  return (
    <ConfigContext.Provider value={{ allowWrites, namespace }}>
      {children}
    </ConfigContext.Provider>
  );
}
