export type GeoIPStatus = {
  db_mtime: string;
  next_scheduled_update: string;
  /** False when no ASN database is loaded, so ASN/IP-type filters cannot apply. */
  asn_available: boolean;
};

export type GeoIPLookupResult = {
  ip: string;
  region: string;
};

export type GeoIPBatchLookupResponse = {
  results: GeoIPLookupResult[];
};

export type GeoIPUpdateResponse = {
  status: string;
};
