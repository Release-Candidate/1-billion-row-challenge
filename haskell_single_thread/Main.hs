-- SPDX-FileCopyrightText:  Copyright 2024 Roland Csaszar
-- SPDX-License-Identifier: MIT
--
-- Project:  1-billion-row-challenge
-- File:     Main.hs
-- Date:     12.Mar.2024
--
--------------------------------------------------------------------------------
{-# LANGUAGE StrictData #-}
{-# LANGUAGE ViewPatterns #-}

module Main (main) where

import Control.Monad (when)
import Data.Array.IO qualified as A
import Data.ByteString.Char8 qualified as BS
import Data.Char (Char, ord)
import Data.Foldable (traverse_)
import Data.HashMap.Strict qualified as HM
import Data.List (sort, splitAt)
import Data.Text.Encoding qualified as TE
import Data.Word (Word64)
import GHC.Float (roundDouble)
import System.Environment (getArgs)
import Text.Printf qualified as T
import Prelude (
  Applicative (pure),
  Bool (True),
  Double,
  Eq ((==)),
  Foldable (null),
  Fractional ((/)),
  IO,
  Int,
  Maybe (Just, Nothing),
  Num (abs, signum, (*), (+), (-)),
  Ord (max, min, (<), (>=)),
  fromIntegral,
  head,
  otherwise,
  ($),
  (&&),
 )

data TemperatureData
  = TemperatureData
      (A.IOUArray Word64 Word64)
      (A.IOUArray Word64 Int)
      (A.IOUArray Word64 Int)
      (A.IOUArray Word64 Int)

parse ::
  Word64 ->
  BS.ByteString ->
  (HM.HashMap BS.ByteString Word64, TemperatureData) ->
  IO (HM.HashMap BS.ByteString Word64, TemperatureData)
parse _ (BS.null -> True) acc = pure acc
parse idx content (accMap, TemperatureData countsT sumT minT maxT) = do
  let (stationName, rest') = BS.break (== ';') content
  let rest2 = BS.drop 1 rest'
  let (temp, rest) = case BS.uncons rest2 of
        Nothing -> (0, rest2)
        Just ('-', rest2') -> parseNegTemp rest2'
        Just (ch1, rest3) -> parseTemp ch1 rest3
  (newAcc, newIdx) <- case HM.lookup stationName accMap of
    Nothing -> do
      A.writeArray countsT idx 1
      A.writeArray sumT idx temp
      A.writeArray minT idx temp
      A.writeArray maxT idx temp
      pure ((HM.insert stationName idx accMap, TemperatureData countsT sumT minT maxT), idx + 1)
    Just idxM -> do
      A.modifyArray' countsT idxM (+ 1)
      A.modifyArray' sumT idxM (+ temp)
      A.modifyArray' minT idxM (min temp)
      A.modifyArray' maxT idxM (max temp)
      pure ((accMap, TemperatureData countsT sumT minT maxT), idx)
  parse newIdx (BS.drop 1 rest) newAcc

{-# INLINE parseNegTemp #-}
parseNegTemp :: BS.ByteString -> (Int, BS.ByteString)
parseNegTemp rest = case BS.uncons rest of
  Nothing -> (0, rest)
  Just (ch1, rest3) -> case BS.uncons rest3 of
    Nothing -> (0, rest3)
    Just ('.', rest4) -> case BS.uncons rest4 of
      Nothing -> (0, rest4)
      Just (ch2, rest5) -> (528 - 10 * ord ch1 - ord ch2, rest5)
    Just (ch2, rest4) -> case BS.uncons rest4 of
      Nothing -> (0, rest4)
      -- Must be '.'
      Just (_, rest5) -> case BS.uncons rest5 of
        Nothing -> (0, rest5)
        Just (ch3, rest6) -> (5328 - 100 * ord ch1 - 10 * ord ch2 - ord ch3, rest6)

{-# INLINE parseTemp #-}
parseTemp :: Char -> BS.ByteString -> (Int, BS.ByteString)
parseTemp ch1 rest = case BS.uncons rest of
  Nothing -> (0, rest)
  Just ('.', rest4) -> case BS.uncons rest4 of
    Nothing -> (0, rest4)
    Just (ch2, rest5) -> (10 * ord ch1 + ord ch2 - 528, rest5)
  Just (ch2, rest4) -> case BS.uncons rest4 of
    Nothing -> (0, rest4)
    -- Must be '.'
    Just (_, rest5) -> case BS.uncons rest5 of
      Nothing -> (0, rest5)
      Just (ch3, rest6) -> (100 * ord ch1 + 10 * ord ch2 + ord ch3 - 5328, rest6)

main :: IO ()
main = do
  args <- getArgs
  when (null args) $ T.perror "Error: no data file to read given! Exiting."
  content <- BS.readFile $ head args
  c <- A.newArray_ (0 :: Word64, 10000 :: Word64)
  s <- A.newArray_ (0 :: Word64, 10000 :: Word64)
  m <- A.newArray_ (0 :: Word64, 10000 :: Word64)
  n <- A.newArray_ (0 :: Word64, 10000 :: Word64)
  (ma, TemperatureData rC rS rM rN) <- parse 0 content (HM.empty, TemperatureData c s m n)

  let keys = sort (HM.keys ma)
  T.printf "{"
  let (e1, els) = splitAt 1 keys
  let i1 = HM.findWithDefault 0 (head e1) ma
  let name1 = TE.decodeUtf8 (head e1)
  count1 <- A.readArray rC i1
  sum1 <- A.readArray rS i1
  tMin1 <- A.readArray rM i1
  tMax1 <- A.readArray rN i1
  let mean1 :: Double = fromIntegral sum1 / fromIntegral count1
  T.printf
    "%s=%.1f/%.1f/%.1f"
    name1
    (roundJava $ fromIntegral tMin1)
    (roundJava mean1)
    (roundJava $ fromIntegral tMax1)
  traverse_
    ( \k ->
        do
          let i = HM.findWithDefault 0 k ma
          let name = TE.decodeUtf8 k
          count <- A.readArray rC i
          sum <- A.readArray rS i
          tMin <- A.readArray rM i
          tMax <- A.readArray rN i
          let mean :: Double = fromIntegral sum / fromIntegral count
          T.printf
            ", %s=%.1f/%.1f/%.1f"
            name
            (roundJava $ fromIntegral tMin)
            (roundJava mean)
            (roundJava $ fromIntegral tMax)
    )
    els
  T.printf "}\n"

{-# INLINE roundJava #-}
roundJava :: Double -> Double
roundJava y =
  let r :: Double = fromIntegral (roundDouble y :: Int)
   in case y of
        x
          | x < 0.0 && r - x == 0.5 -> r / 10.0
          | abs (x - r) >= 0.5 -> (r + signum y) / 10.0
          | otherwise -> r / 10.0
