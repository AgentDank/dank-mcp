-- Brings up our DuckDB database

CREATE TABLE IF NOT EXISTS brands_us_ct (
    brand_name TEXT,
    dosage_form TEXT,
    branding_entity TEXT,
    product_image_url TEXT,
    product_image_desc TEXT,
    label_image_url TEXT,
    lavel_image_desc TEXT,
    lab_analysis_url TEXT,
    lab_analysis_desc TEXT,
    approval_date DATETIME,
    registration_number TEXT NOT NULL,
    tetrahydrocannabinol_thc DOUBLE,
    tetrahydrocannabinol_acid_thca DOUBLE,
    cannabidiols_cbd DOUBLE,
    cannabidiol_acid_cbda DOUBLE,
    a_pinene DOUBLE,
    b_myrcene DOUBLE,
    b_caryophyllene DOUBLE,
    b_pinene DOUBLE,
    limonene DOUBLE,
    ocimene DOUBLE,
    linalool_lin DOUBLE,
    humulene_hum DOUBLE,
    cbg DOUBLE,
    cbg_a DOUBLE,
    cannabavarin_cbdv DOUBLE,
    cannabichromene_cbc DOUBLE,
    cannbinol_cbn DOUBLE,
    tetrahydrocannabivarin_thcv DOUBLE,
    a_bisabolol DOUBLE,
    a_phellandrene DOUBLE,
    a_terpinene DOUBLE,
    b_eudesmol DOUBLE,
    b_terpinene DOUBLE,
    fenchone DOUBLE,
    pulegol DOUBLE,
    borneol DOUBLE,
    isopulegol DOUBLE,
    carene DOUBLE,
    camphene DOUBLE,
    camphor DOUBLE,
    caryophyllene_oxide DOUBLE,
    cedrol DOUBLE,
    eucalyptol DOUBLE,
    geraniol DOUBLE,
    guaiol DOUBLE,
    geranyl_acetate DOUBLE,
    isoborneol DOUBLE,
    menthol DOUBLE,
    l_fenchone DOUBLE,
    nerol DOUBLE,
    sabinene DOUBLE,
    terpineol DOUBLE,
    terpinolene DOUBLE,
    trans_b_farnesene DOUBLE,
    valencene DOUBLE,
    a_cedrene DOUBLE,
    a_farnesene DOUBLE,
    b_farnesene DOUBLE,
    cis_nerolidol DOUBLE,
    fenchol DOUBLE,
    trans_nerolidol DOUBLE,
    market TEXT,
    chemotype TEXT,
    processing_technique TEXT,
    solvents_used TEXT,
    national_drug_code TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS brands_us_ct_reg ON brands_us_ct (registration_number);
CREATE UNIQUE INDEX IF NOT EXISTS brands_us_ct_brand ON brands_us_ct (brand_name);
CREATE INDEX IF NOT EXISTS brands_us_ct_date_brand ON brands_us_ct (approval_date, brand_name);
