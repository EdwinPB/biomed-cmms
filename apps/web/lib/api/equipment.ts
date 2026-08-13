import { api } from "./client";
import { EquipmentList, SelectableEquipmentList } from "../types/api";

export function listEquipment(): Promise<EquipmentList> {
  return api.get<EquipmentList>("/api/v1/equipment");
}

export function listSelectableEquipment(): Promise<SelectableEquipmentList> {
  return api.get<SelectableEquipmentList>("/api/v1/equipment/selectable");
}
