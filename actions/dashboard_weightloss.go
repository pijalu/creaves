package actions

import (
	"creaves/models"
	"fmt"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

const ANIMALS_WITH_WEIGTHLOSS = `
WITH ParsedWeights AS (
    SELECT
        animal_id,
        date,
        CAST(NULLIF(weight, '') AS DECIMAL(10,2)) AS weight_in_grams
    FROM cares 
    WHERE date >= DATE_SUB(CURDATE(), INTERVAL 7 day) -- Filter last 7 days
      AND weight is not null and weight <> "" and (CAST(NULLIF(weight, '') AS DECIMAL(10,2))) is not null
),
RankedWeights AS (
    SELECT
        animal_id,
        date,
        weight_in_grams,
		ROW_NUMBER() OVER (PARTITION BY animal_id ORDER BY date DESC) AS recent_rank,
        ROW_NUMBER() OVER (PARTITION BY animal_id ORDER BY date ASC) AS oldest_rank
    FROM ParsedWeights
)
SELECT 
	a.*
	-- a.year, a.yearNumber, a.cage, -- t1.*, t2.*
    -- t1.animal_id, t1.date, t1.weight_in_grams, t2.date, t2.weight_in_grams, t1.weight_in_grams - t2.weight_in_grams
FROM RankedWeights t1
JOIN RankedWeights t2
    ON t1.animal_id = t2.animal_id
JOIN animals a
	ON t1.animal_id = a.ID
WHERE t1.recent_rank = 1  -- Most recent weight
  AND t2.oldest_rank = 1  -- oldest recent weight
  AND t1.weight_in_grams IS NOT NULL
  AND t2.weight_in_grams IS NOT NULL
  AND t1.weight_in_grams <= t2.weight_in_grams * 0.9 -- Weight decreased by 10%
  AND a.outtake_id is null
`

func listAnimalWithWeightLoss(c buffalo.Context) (*models.Animals, error) {
	animals := &models.Animals{}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	// Retrieve all animals with today treatments from the DB
	if err := tx.RawQuery(ANIMALS_WITH_WEIGTHLOSS).All(animals); err != nil {
		return nil, err
	}
	return EnrichAnimals(animals, c)
}
