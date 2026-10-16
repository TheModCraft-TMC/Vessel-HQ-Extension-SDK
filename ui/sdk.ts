/** Native UI receives only host-scoped context and the permission-checked broker. */
export interface StackDetailContext {
  environmentId: number;
  stackId: number;
  stackName: string;
}

export interface ExtensionNote {
  text: string;
  version: number;
}

export interface NativeExtensionStorage {
  read(): Promise<ExtensionNote>;
  write(note: ExtensionNote): Promise<ExtensionNote>;
}

export interface NativeExtensionProps {
  watchTraffic?(
    onSnapshot: (snapshot: StackTrafficOverview) => void,
    signal: AbortSignal
  ): Promise<void>;
  storage?: NativeExtensionStorage;
  context: Readonly<StackDetailContext>;
  invoke<T>(
    operation: 'stacks.network.read' | 'stacks.traffic.read',
    input: { environmentId: number; stackId: number }
  ): Promise<T>;
}

export interface StackNetworkOverview {
  stackId: number;
  environmentId: number;
  name: string;
  networks: {
    id: string;
    name: string;
    driver: string;
    scope: string;
    internal: boolean;
  }[];
  workloads: {
    id: string;
    name: string;
    kind: 'container' | 'service';
    state?: string;
    networks: { networkId: string }[];
  }[];
}

export interface StackTrafficOverview extends StackNetworkOverview {
  traffic: {
    collectedAt: string;
    routers: {
      id: string;
      protocol: string;
      workloadId: string;
      rule: string;
      service: string;
      entryPoints: string[];
      middlewares: string[];
      tls: boolean;
    }[];
    services: {
      id: string;
      protocol: string;
      workloadId: string;
      port: number;
      scheme: string;
    }[];
    middlewares: {
      id: string;
      protocol: string;
      workloadId: string;
      types: string[];
      settings: Record<string, string>;
      redacted: boolean;
    }[];
    counters: {
      workloadId: string;
      receivedBytes: string;
      sentBytes: string;
      available: boolean;
      reason: string;
    }[];
    warnings: string[];
  };
}
