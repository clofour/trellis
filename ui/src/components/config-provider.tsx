"use client";

import { createContext, useContext } from "react";

interface Config {
  allowWrites: boolean;
}

const ConfigContext = createContext<Config>({ allowWrites: false });

export function useConfig(): Config {
  return useContext(ConfigContext);
}

export function ConfigProvider({
  allowWrites,
  children,
}: {
  allowWrites: boolean;
  children: React.ReactNode;
}) {
  return (
    <ConfigContext.Provider value={{ allowWrites }}>
      {children}
    </ConfigContext.Provider>
  );
}
