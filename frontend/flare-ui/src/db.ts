import Dexie, { type Table } from 'dexie'

export interface Counter {
  id?: number
  value: number
}

export class FlareDB extends Dexie {
  counter!: Table<Counter>

  constructor() {
    super('flare-db')
    this.version(1).stores({
      counter: '++id, value'
    })
  }
}

export const db = new FlareDB()
