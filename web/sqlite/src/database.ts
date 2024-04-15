// import initSqlJs from "@stephen/sql.js";
import { Database } from "@stephen/sql.js";
import { Buffer } from "buffer";

// // Required to let webpack 4 know it needs to copy the wasm file to our assets
// // eslint-disable-next-line import/no-webpack-loader-syntax
// // import sqlWasm from "!!file-loader?name=sql-wasm-[contenthash].wasm!sql.js/dist/sql-wasm.wasm";
// import sqlWasm from '@stephen/sql.js/dist/sql-wasm.wasm';

let db: Database;
let SQL: { Database: any; default?: any; };

export async function importDB(url: URL) {
  const response = await fetch(url);
  const dbData = await response.arrayBuffer();
  db = new SQL.Database(Buffer.from(dbData));
}

export async function initDB() {
  SQL = await import("@stephen/sql.js");
}

export function getDB() {
  return db;
}

export {Database};
