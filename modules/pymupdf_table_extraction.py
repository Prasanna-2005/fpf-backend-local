import json
import os
import sys
from statistics import median

import fitz
import pandas as pd


class PymupdfTableExtraction:
    def extract_blocks(page_obj):
        return page_obj.extractWORDS()

    def load_page_text(file_path):
        doc = fitz.open(file_path)
        page_dfs = []
        for page in doc:
            obj = page.get_textpage()
            word_data = PymupdfTableExtraction.extract_blocks(obj)
            table_df = PymupdfTableExtraction.convert_to_table(word_data)
            if not table_df.empty:
                page_dfs.append(table_df)
        return page_dfs

    def _filter_central_region(word_data, width_ratio=0.5, min_words=5):
        """Keep only words whose horizontal centers fall inside the central band."""
        if not word_data:
            return word_data

        x_centers = [(entry[0] + entry[2]) / 2 for entry in word_data]
        if not x_centers:
            return word_data

        min_x, max_x = min(x_centers), max(x_centers)
        center = (min_x + max_x) / 2
        tolerance = max(1, (max_x - min_x) * width_ratio * 0.5)

        filtered = [
            entry
            for entry, x_center in zip(word_data, x_centers)
            if abs(x_center - center) <= tolerance
        ]

        return filtered if len(filtered) >= min_words else word_data

    def _find_table_bounds(
        word_data,
        rows,
        width_ratio=0.45,
        min_words_per_row=3,
        min_consecutive_rows=2,
    ):
        """Return the vertical bounds for the densest block of table rows."""
        if not word_data or not rows:
            return None

        page_min_x = min(entry[0] for entry in word_data)
        page_max_x = max(entry[2] for entry in word_data)
        width_threshold = (page_max_x - page_min_x) * width_ratio
        width_threshold = max(width_threshold, 1)

        best_block = None
        best_len = 0
        current_start = None

        def row_width(row):
            min_x = min(word["x0"] for word in row["words"])
            max_x = max(word["x1"] for word in row["words"])
            return max_x - min_x

        for idx, row in enumerate(rows):
            width = row_width(row)
            is_table_row = (
                width >= width_threshold and len(row["words"]) >= min_words_per_row
            )

            if is_table_row:
                if current_start is None:
                    current_start = idx
            else:
                if current_start is not None:
                    block_len = idx - current_start
                    if block_len > best_len:
                        best_len = block_len
                        best_block = (current_start, idx - 1)
                    current_start = None
        if current_start is not None:
            block_len = len(rows) - current_start
            if block_len > best_len:
                best_len = block_len
                best_block = (current_start, len(rows) - 1)

        if not best_block or best_len < min_consecutive_rows:
            return None

        selected_rows = rows[best_block[0] : best_block[1] + 1]
        top = min(word["y0"] for row in selected_rows for word in row["words"])
        bottom = max(word["y1"] for row in selected_rows for word in row["words"])
        return top, bottom

    def _filter_by_vertical_bounds(word_data, bounds):
        """Keep only words whose vertical centers fall within the specified bounds."""
        if not bounds:
            return word_data
        top, bottom = bounds
        filtered = []
        for entry in word_data:
            center_y = (entry[1] + entry[3]) / 2
            if top <= center_y <= bottom:
                filtered.append(entry)
        return filtered

    def identify_rows(word_data):
        """Identify rows in the word data based on vertical centers."""
        heights = [entry[3] - entry[1] for entry in word_data]
        median_height = median(heights) if heights else 0
        row_tolerance = max(1, median_height * 0.5)

        rows = []
        for x0, y0, x1, y1, word, block_no, line_no, word_no in sorted(
            word_data, key=lambda w: w[1]
        ):
            center_y = (y0 + y1) / 2
            matched_row = next(
                (
                    row
                    for row in rows
                    if abs(row["center_y"] - center_y) <= row_tolerance
                ),
                None,
            )
            word_entry = {"text": word, "x0": x0, "x1": x1, "y0": y0, "y1": y1}
            if matched_row:
                matched_row["words"].append(word_entry)
                matched_row["center_y"] = (
                    matched_row["center_y"] * matched_row["count"] + center_y
                ) / (matched_row["count"] + 1)
                matched_row["count"] += 1
            else:
                rows.append({"center_y": center_y, "words": [word_entry], "count": 1})

        rows.sort(key=lambda r: r["center_y"])

        return rows

    def identify_columns(word_data):
        """Identify columns in the word data based on horizontal centers."""
        widths = [entry[2] - entry[0] for entry in word_data]
        median_width = median(widths) if widths else 0
        column_tolerance = max(1, median_width * 0.7)

        x_centers = sorted((entry[0] + entry[2]) / 2 for entry in word_data)
        columns = []
        for x_center in x_centers:
            placed = False
            for col in columns:
                if abs(col["center"] - x_center) <= column_tolerance:
                    col["values"].append(x_center)
                    col["center"] = sum(col["values"]) / len(col["values"])
                    placed = True
                    break
            if not placed:
                columns.append({"center": x_center, "values": [x_center]})

        columns = sorted(columns, key=lambda col: col["center"])
        column_centers = [col["center"] for col in columns] or [0]

        return column_centers

    @staticmethod
    def extract_table_title(word_data, bounds, page_width):
        if not bounds:
            return None
        above_words = [e for e in word_data if e[3] < bounds[0]]
        rows = PymupdfTableExtraction.identify_rows(above_words)
        page_center = page_width / 2
        titles = [
            " ".join(w["text"] for w in sorted(r["words"], key=lambda w: w["x0"]))
            for r in rows[-5:]
            if abs(
                (min(w["x0"] for w in r["words"]) + max(w["x1"] for w in r["words"]))
                / 2
                - page_center
            )
            <= page_width * 0.15
        ]
        return " | ".join(titles) if titles else None

    def convert_to_table(word_data):
        """Convert word data to a table dataframe."""
        initial_rows = PymupdfTableExtraction.identify_rows(word_data)
        bounds = PymupdfTableExtraction._find_table_bounds(word_data, initial_rows)
        filtered_data = (
            PymupdfTableExtraction._filter_by_vertical_bounds(word_data, bounds)
            if bounds
            else PymupdfTableExtraction._filter_central_region(word_data)
        )
        rows = PymupdfTableExtraction.identify_rows(filtered_data)
        column_centers = PymupdfTableExtraction.identify_columns(filtered_data)

        data = []
        for row in rows:
            cell_blocks = {idx: [] for idx in range(len(column_centers))}
            for word_entry in row["words"]:
                x_center = (word_entry["x0"] + word_entry["x1"]) / 2
                closest_idx, min_diff = None, float("inf")
                for idx, center in enumerate(column_centers):
                    diff = abs(center - x_center)
                    if diff < min_diff:
                        min_diff = diff
                        closest_idx = idx
                if closest_idx is not None:
                    cell_blocks[closest_idx].append(word_entry["text"])
            row_values = [
                " ".join(cell_blocks[idx]).strip() for idx in range(len(column_centers))
            ]
            data.append(row_values)

        column_names = [f"col_{idx + 1}" for idx in range(len(column_centers))]
        df = pd.DataFrame(data, columns=column_names)
        df = df.loc[~(df == "").all(axis=1)].reset_index(drop=True)
        df = PymupdfTableExtraction._merge_split_columns(df)
        return df

    @staticmethod
    def _get_first_row_text(series):
        """Return the stripped value appearing in the first row of a series."""
        if series.empty:
            return ""
        value = series.iat[0]
        return "" if pd.isna(value) else str(value).strip()

    @staticmethod
    def _is_header_only_column(series, max_header_rows=3):
        """Return True if the column appears to contain only header fragments."""
        non_empty = [
            str(value).strip() for value in series.dropna() if str(value).strip()
        ]
        if not non_empty:
            return True
        if any(char.isdigit() for value in non_empty for char in value):
            return False
        return len(non_empty) <= max_header_rows

    @staticmethod
    def _merge_split_columns(df):
        """Merge adjacent columns that store multi-line headers before the actual data."""
        if df.empty:
            return df

        merged = pd.DataFrame()
        header_parts = []
        for column in df.columns:
            series = df[column]
            header_text = PymupdfTableExtraction._get_first_row_text(series)
            if PymupdfTableExtraction._is_header_only_column(series):
                if header_text:
                    header_parts.append(header_text)
                continue

            if header_text:
                header_parts.append(header_text)

            column_name = " ".join(part for part in header_parts if part).strip()
            if not column_name:
                column_name = column

            merged[column_name] = series
            header_parts = []

        return merged if not merged.empty else df


if __name__ == "__main__":
    data = json.load(sys.stdin)
    pdf_paths = data["pdf_paths"]
    excel_dir = data["excel_dir"]
    os.makedirs(excel_dir, exist_ok=True)
    for file_path in pdf_paths:
        page_dfs = PymupdfTableExtraction.load_page_text(file_path)
        if not page_dfs:
            continue
        base_name = os.path.splitext(os.path.basename(file_path))[0]
        workbook_path = os.path.join(excel_dir, f"{base_name}.xlsx")
        with pd.ExcelWriter(workbook_path) as writer:
            for page_index, page_df in enumerate(page_dfs, start=1):
                page_df.to_excel(writer, sheet_name=f"Page {page_index}", index=False)
