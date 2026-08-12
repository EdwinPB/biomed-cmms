import { api } from "./client";
import { EquipmentList } from "../types/api";

export function listEquipment(): Promise<EquipmentList> {
  return api.get<EquipmentList>("/api/v1/equipment");
}
