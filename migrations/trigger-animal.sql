USE creaves;
DELIMITER $$

CREATE TRIGGER before_animals_insert
BEFORE INSERT
ON animals FOR EACH ROW
BEGIN
    DECLARE yearNumber INT;
    
    SET NEW.year = YEAR(NEW.created_at);
    
    SELECT MAX(yearNumber)+1 INTO yearNumber
    FROM ANIMALS where YEAR = NEW.year;
    
    SET NEW.yearNumber = yearNumber;

END $$

DELIMITER ;
